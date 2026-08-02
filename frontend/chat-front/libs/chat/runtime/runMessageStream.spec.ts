import { beforeEach, describe, expect, it, vi } from 'vitest'
import { streamMessage } from '@libs/chat/api/chatApi'
import type { ChatStreamEvent } from '@libs/chat/model/types'
import { runMessageStream } from './runMessageStream'
import {
    handleChunkEvent,
    handleDoneEvent,
    handleErrorEvent,
    handleStatusEvent,
} from './streamEventHandlers'

vi.mock('@libs/chat/api/chatApi', () => ({
    streamMessage: vi.fn(),
}))

vi.mock('./streamEventHandlers', () => ({
    handleStatusEvent: vi.fn(),
    handleChunkEvent: vi.fn(),
    handleDoneEvent: vi.fn(),
    handleErrorEvent: vi.fn(),
}))

type RunMessageStreamParams = Parameters<typeof runMessageStream>[0]

const createParams = (
    overrides: Partial<RunMessageStreamParams> = {},
): RunMessageStreamParams => ({
    initialChatId: 'initial-chat',
    prompt: 'Hello',
    model: 'test-model',
    chatIdRef: { current: 'chat-1' },
    pendingNavigationChatIdRef: { current: null },
    setMessages: vi.fn(),
    navigateToChat: vi.fn(),
    signal: undefined,
    ...overrides,
})

const createStream = (events: ChatStreamEvent[]) =>
    (async function* (): AsyncGenerator<ChatStreamEvent, void, unknown> {
        for (const event of events) {
            yield event
        }
    })()

const mockStream = (events: ChatStreamEvent[]) => {
    vi.mocked(streamMessage).mockResolvedValue(createStream(events))
}

const flushMicrotasks = () => new Promise<void>((resolve) => setTimeout(resolve, 0))

const statusEvent: ChatStreamEvent = { type: 'status', stage: 'started', chat_id: 'chat-1' }

const doneEvent: ChatStreamEvent = {
    type: 'done',
    status: 'completed',
    tokens_used: { prompt: 1, completion: 2, total: 3 },
    finished_at: '2026-08-02T10:00:00Z',
}

const errorEvent: ChatStreamEvent = {
    type: 'error',
    code: 'STREAM_FAILED',
    message: 'Something went wrong',
    retry_allowed: false,
}

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(handleChunkEvent).mockImplementation((event, textContent) => textContent + event.content)
    vi.mocked(handleDoneEvent).mockResolvedValue(undefined)
})

describe('runMessageStream', () => {
    it('calls streamMessage with current chat id, prompt, model and signal', async () => {
        const controller = new AbortController()
        mockStream([])

        await runMessageStream(
            createParams({ chatIdRef: { current: 'chat-42' }, signal: controller.signal }),
        )

        expect(streamMessage).toHaveBeenCalledWith(
            'chat-42',
            'Hello',
            'test-model',
            controller.signal,
        )
    })

    it('calls handleStatusEvent with the event and the stream context', async () => {
        const params = createParams()
        mockStream([statusEvent])

        await runMessageStream(params)

        expect(handleStatusEvent).toHaveBeenCalledTimes(1)
        expect(handleStatusEvent).toHaveBeenCalledWith(statusEvent, {
            initialChatId: 'initial-chat',
            chatIdRef: params.chatIdRef,
            pendingNavigationChatIdRef: params.pendingNavigationChatIdRef,
            setMessages: params.setMessages,
            navigateToChat: params.navigateToChat,
        })
    })

    it('accumulates text content across multiple chunk events', async () => {
        const chunks: ChatStreamEvent[] = [
            { type: 'chunk', content: 'Hello' },
            { type: 'chunk', content: ' world' },
            { type: 'chunk', content: '!' },
        ]
        mockStream(chunks)

        await runMessageStream(createParams())

        expect(handleChunkEvent).toHaveBeenCalledTimes(3)
        expect(handleChunkEvent).toHaveBeenNthCalledWith(1, chunks[0], '', expect.anything())
        expect(handleChunkEvent).toHaveBeenNthCalledWith(2, chunks[1], 'Hello', expect.anything())
        expect(handleChunkEvent).toHaveBeenNthCalledWith(
            3,
            chunks[2],
            'Hello world',
            expect.anything(),
        )
    })

    it('calls handleDoneEvent with the stream context', async () => {
        const params = createParams()
        mockStream([doneEvent])

        await runMessageStream(params)

        expect(handleDoneEvent).toHaveBeenCalledTimes(1)
        expect(handleDoneEvent).toHaveBeenCalledWith({
            initialChatId: 'initial-chat',
            chatIdRef: params.chatIdRef,
            pendingNavigationChatIdRef: params.pendingNavigationChatIdRef,
            setMessages: params.setMessages,
            navigateToChat: params.navigateToChat,
        })
    })

    it('waits for handleDoneEvent to settle before finishing', async () => {
        let resolveDone!: () => void
        vi.mocked(handleDoneEvent).mockReturnValue(
            new Promise<void>((resolve) => {
                resolveDone = resolve
            }),
        )
        mockStream([doneEvent])

        let finished = false
        const pending = runMessageStream(createParams()).then(() => {
            finished = true
        })

        await flushMicrotasks()

        expect(handleDoneEvent).toHaveBeenCalledTimes(1)
        expect(finished).toBe(false)

        resolveDone()
        await pending

        expect(finished).toBe(true)
    })

    it('propagates the error thrown by handleErrorEvent', async () => {
        vi.mocked(handleErrorEvent).mockImplementation(() => {
            throw new Error('Something went wrong')
        })
        mockStream([errorEvent])

        await expect(runMessageStream(createParams())).rejects.toThrow('Something went wrong')

        expect(handleErrorEvent).toHaveBeenCalledWith(errorEvent)
    })

    it('stops processing events once the signal is aborted', async () => {
        const controller = new AbortController()
        vi.mocked(streamMessage).mockResolvedValue(
            (async function* (): AsyncGenerator<ChatStreamEvent, void, unknown> {
                yield { type: 'chunk', content: 'first' }
                controller.abort()
                yield { type: 'chunk', content: 'second' }
                yield doneEvent
            })(),
        )

        await runMessageStream(createParams({ signal: controller.signal }))

        expect(handleChunkEvent).toHaveBeenCalledTimes(1)
        expect(handleChunkEvent).toHaveBeenCalledWith(
            { type: 'chunk', content: 'first' },
            '',
            expect.anything(),
        )
        expect(handleDoneEvent).not.toHaveBeenCalled()
    })

    it('handles a mixed event stream in order', async () => {
        const events: ChatStreamEvent[] = [
            statusEvent,
            { type: 'chunk', content: 'Hello' },
            { type: 'chunk', content: ' world' },
            doneEvent,
        ]
        mockStream(events)

        await runMessageStream(createParams())

        expect(handleStatusEvent).toHaveBeenCalledTimes(1)
        expect(handleChunkEvent).toHaveBeenCalledTimes(2)
        expect(handleDoneEvent).toHaveBeenCalledTimes(1)
        expect(handleErrorEvent).not.toHaveBeenCalled()

        const statusOrder = vi.mocked(handleStatusEvent).mock.invocationCallOrder[0]
        const chunkOrders = vi.mocked(handleChunkEvent).mock.invocationCallOrder
        const doneOrder = vi.mocked(handleDoneEvent).mock.invocationCallOrder[0]

        expect(statusOrder).toBeLessThan(chunkOrders[0])
        expect(chunkOrders[0]).toBeLessThan(chunkOrders[1])
        expect(chunkOrders[1]).toBeLessThan(doneOrder)
    })
})
