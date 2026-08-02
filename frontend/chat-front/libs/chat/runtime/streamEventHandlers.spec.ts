import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ThreadMessageLike } from '@assistant-ui/react'
import { notifyChatsUpdated } from '@libs/chat/api/chatApi'
import {
    handleChunkEvent,
    handleDoneEvent,
    handleErrorEvent,
    handleStatusEvent,
} from './streamEventHandlers'

vi.mock('@libs/chat/api/chatApi', () => ({
    notifyChatsUpdated: vi.fn(),
}))

vi.mock('react-dom', () => ({
    flushSync: (callback: () => void) => callback(),
}))

type StreamHandlerContext = Parameters<typeof handleDoneEvent>[0]

const createContext = (overrides: Partial<StreamHandlerContext> = {}): StreamHandlerContext => ({
    initialChatId: undefined,
    chatIdRef: { current: '' },
    pendingNavigationChatIdRef: { current: null },
    setMessages: vi.fn(),
    navigateToChat: vi.fn(),
    ...overrides,
})

const createStatusEvent = (chatId: string) => ({
    type: 'status' as const,
    stage: 'started',
    chat_id: chatId,
})

beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(console, 'log').mockImplementation(() => {})
})

describe('streamEventHandlers', () => {
    describe('handleStatusEvent', () => {
        it('updates chatIdRef with event chat id', () => {
            const context = createContext({ chatIdRef: { current: 'old-chat' } })

            handleStatusEvent(createStatusEvent('new-chat'), context)

            expect(context.chatIdRef.current).toBe('new-chat')
        })

        it('sets pendingNavigationChatIdRef for a new chat without initialChatId', () => {
            const context = createContext()

            handleStatusEvent(createStatusEvent('new-chat'), context)

            expect(context.pendingNavigationChatIdRef.current).toBe('new-chat')
        })

        it('keeps pendingNavigationChatIdRef empty when initialChatId exists', () => {
            const context = createContext({ initialChatId: 'existing-chat' })

            handleStatusEvent(createStatusEvent('existing-chat'), context)

            expect(context.chatIdRef.current).toBe('existing-chat')
            expect(context.pendingNavigationChatIdRef.current).toBeNull()
        })

        it('does nothing without event chat id', () => {
            const context = createContext({ chatIdRef: { current: 'old-chat' } })

            handleStatusEvent(createStatusEvent(''), context)

            expect(context.chatIdRef.current).toBe('old-chat')
            expect(context.pendingNavigationChatIdRef.current).toBeNull()
        })
    })

    describe('handleChunkEvent', () => {
        it('concatenates chunk content with accumulated text', () => {
            const context = createContext()

            const result = handleChunkEvent({ type: 'chunk', content: ' world' }, 'Hello', context)

            expect(result).toBe('Hello world')
        })

        it('concatenates chunks across sequential calls', () => {
            const context = createContext()

            const first = handleChunkEvent({ type: 'chunk', content: 'He' }, '', context)
            const second = handleChunkEvent({ type: 'chunk', content: 'llo' }, first, context)

            expect(second).toBe('Hello')
        })

        it('calls setMessages with an upsertAssistantText updater', () => {
            const context = createContext()

            handleChunkEvent({ type: 'chunk', content: ' world' }, 'Hello', context)

            expect(context.setMessages).toHaveBeenCalledTimes(1)

            const updater = vi.mocked(context.setMessages).mock.calls[0][0] as (
                prev: readonly ThreadMessageLike[],
            ) => ThreadMessageLike[]
            const previousMessages: ThreadMessageLike[] = [
                { role: 'user', content: [{ type: 'text', text: 'Hi' }] },
            ]

            expect(updater(previousMessages)).toEqual([
                { role: 'user', content: [{ type: 'text', text: 'Hi' }] },
                { role: 'assistant', content: [{ type: 'text', text: 'Hello world' }] },
            ])
        })

        it('replaces existing assistant text through the updater', () => {
            const context = createContext()

            handleChunkEvent({ type: 'chunk', content: ' world' }, 'Hello', context)

            const updater = vi.mocked(context.setMessages).mock.calls[0][0] as (
                prev: readonly ThreadMessageLike[],
            ) => ThreadMessageLike[]
            const previousMessages: ThreadMessageLike[] = [
                { role: 'user', content: [{ type: 'text', text: 'Hi' }] },
                { role: 'assistant', content: [{ type: 'text', text: 'Hello' }] },
            ]

            expect(updater(previousMessages)).toEqual([
                { role: 'user', content: [{ type: 'text', text: 'Hi' }] },
                { role: 'assistant', content: [{ type: 'text', text: 'Hello world' }] },
            ])
        })
    })

    describe('handleDoneEvent', () => {
        it('navigates to the pending chat id and clears the ref', async () => {
            const context = createContext({
                chatIdRef: { current: 'chat-1' },
                pendingNavigationChatIdRef: { current: 'chat-1' },
            })

            await handleDoneEvent(context)

            expect(context.navigateToChat).toHaveBeenCalledWith('chat-1')
            expect(context.pendingNavigationChatIdRef.current).toBeNull()
        })

        it('skips navigation without a pending chat id', async () => {
            const context = createContext({ chatIdRef: { current: 'chat-1' } })

            await handleDoneEvent(context)

            expect(context.navigateToChat).not.toHaveBeenCalled()
        })

        it('notifies chats updated after navigation finishes', async () => {
            let resolveNavigation!: () => void
            const navigateToChat = vi.fn(
                () =>
                    new Promise<void>((resolve) => {
                        resolveNavigation = resolve
                    }),
            )
            const context = createContext({
                chatIdRef: { current: 'chat-2' },
                pendingNavigationChatIdRef: { current: 'chat-2' },
                navigateToChat,
            })

            const pending = handleDoneEvent(context)

            expect(navigateToChat).toHaveBeenCalledWith('chat-2')
            expect(notifyChatsUpdated).not.toHaveBeenCalled()

            resolveNavigation()
            await pending

            expect(notifyChatsUpdated).toHaveBeenCalledWith('chat-2')
        })

        it('notifies chats updated with the current chat id without navigation', async () => {
            const context = createContext({ chatIdRef: { current: 'chat-3' } })

            await handleDoneEvent(context)

            expect(notifyChatsUpdated).toHaveBeenCalledWith('chat-3')
        })
    })

    describe('handleErrorEvent', () => {
        it('throws an error with the event message', () => {
            expect(() =>
                handleErrorEvent({
                    type: 'error',
                    code: 'STREAM_FAILED',
                    message: 'Something went wrong',
                    retry_allowed: false,
                }),
            ).toThrow(new Error('Something went wrong'))
        })
    })
})
