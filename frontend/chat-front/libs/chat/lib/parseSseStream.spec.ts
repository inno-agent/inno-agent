import { describe, it, expect } from 'vitest'
import { parseSseStream } from './parseSseStream'
import type { ChatStreamEvent } from '@libs/chat/model/types'

const encoder = new TextEncoder()

function createResponse(chunks: Array<string | Error>): Response {
    let index = 0
    const reader = {
        read: () => {
            if (index >= chunks.length) {
                return Promise.resolve({ done: true, value: undefined })
            }
            const chunk = chunks[index++]
            if (chunk instanceof Error) {
                return Promise.reject(chunk)
            }
            return Promise.resolve({ done: false, value: encoder.encode(chunk) })
        },
        releaseLock: () => {},
        cancel: () => Promise.resolve(),
    }
    return { body: { getReader: () => reader } } as unknown as Response
}

async function collect(response: Response): Promise<ChatStreamEvent[]> {
    const events: ChatStreamEvent[] = []
    for await (const event of parseSseStream(response)) {
        events.push(event)
    }
    return events
}

describe('parseSseStream', () => {
    it('throws when response body has no reader', async () => {
        const response = { body: null } as unknown as Response
        await expect(collect(response)).rejects.toThrow('No reader available')
    })

    it('parses a normal SSE event into a ChatStreamEvent', async () => {
        const response = createResponse(['event: chunk\ndata: {"content":"Hello"}\n\n'])
        const events = await collect(response)
        expect(events).toEqual([{ type: 'chunk', content: 'Hello' }])
    })

    it('defaults type to message when event name is absent', async () => {
        const response = createResponse(['data: {"content":"Hi"}\n\n'])
        const events = await collect(response)
        expect(events).toEqual([{ type: 'message', content: 'Hi' }])
    })

    it('ignores malformed JSON without breaking the stream', async () => {
        const response = createResponse([
            'data: {invalid}\n\n',
            'event: chunk\ndata: {"content":"ok"}\n\n',
        ])
        const events = await collect(response)
        expect(events).toEqual([{ type: 'chunk', content: 'ok' }])
    })

    it('completes the generator on [DONE]', async () => {
        const response = createResponse([
            'event: chunk\ndata: {"content":"first"}\n\n',
            'data: [DONE]\n\n',
            'event: chunk\ndata: {"content":"after"}\n\n',
        ])
        const events = await collect(response)
        expect(events).toEqual([{ type: 'chunk', content: 'first' }])
    })

    it('ends cleanly when reader is done with no events', async () => {
        const response = createResponse([])
        const events = await collect(response)
        expect(events).toEqual([])
    })

    it('completes silently on AbortError', async () => {
        const abortError = new DOMException('Aborted', 'AbortError')
        const response = createResponse([
            'event: chunk\ndata: {"content":"partial"}\n\n',
            abortError,
        ])
        const events: ChatStreamEvent[] = []
        const consume = (async () => {
            for await (const event of parseSseStream(response)) {
                events.push(event)
            }
        })()
        const timeout = new Promise<never>((_, reject) => {
            setTimeout(() => reject(new Error('parseSseStream did not settle')), 1000)
        })
        await expect(Promise.race([consume, timeout])).resolves.toBeUndefined()
        expect(events).toEqual([{ type: 'chunk', content: 'partial' }])
    })
})
