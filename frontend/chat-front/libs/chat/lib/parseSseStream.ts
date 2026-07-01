import { createParser, type EventSourceMessage } from 'eventsource-parser'
import type { ChatStreamEvent } from '@libs/chat/model/types'

interface AsyncQueueItem<T> {
    done: boolean
    value?: T
}


export async function* parseSseStream(
    response: Response,
): AsyncGenerator<ChatStreamEvent, void, unknown> {
    const reader = response.body?.getReader()
    if (!reader) throw new Error('No reader available')

    const decoder = new TextDecoder()
    const bufferedItems: AsyncQueueItem<ChatStreamEvent>[] = []
    let pendingResolve: ((item: AsyncQueueItem<ChatStreamEvent>) => void) | null = null
    let streamClosed = false

    const pushItem = (item: AsyncQueueItem<ChatStreamEvent>) => {
        if (pendingResolve) {
            const resolve = pendingResolve
            pendingResolve = null
            resolve(item)
            return
        }

        bufferedItems.push(item)
    }

    const nextItem = () => {
        const item = bufferedItems.shift()
        if (item) {
            return Promise.resolve(item)
        }

        return new Promise<AsyncQueueItem<ChatStreamEvent>>((resolve) => {
            pendingResolve = resolve
        })
    }

    const parser = createParser({
        onEvent: (event: EventSourceMessage) => {
            if (event.data === '[DONE]') {
                streamClosed = true
                pushItem({ done: true })
                return
            }

            try {
                const payload = JSON.parse(event.data)
                const streamEvent = {
                    type: event.event ?? 'message',
                    ...payload,
                } as ChatStreamEvent

                pushItem({
                    done: false,
                    value: streamEvent,
                })
            } catch {
                // Failed to parse SSE event
            }
        },
    })

    const pump = (async () => {
        try {
            while (!streamClosed) {
                const result = await reader.read()
                if (result.done) {
                    break
                }

                parser.feed(decoder.decode(result.value, { stream: true }))
            }
        } catch (error) {
            if (error instanceof DOMException && error.name === 'AbortError') {
                streamClosed = true
            } else {
                throw error
            }
        } finally {
            reader.releaseLock()
            if (!streamClosed) {
                streamClosed = true
                pushItem({ done: true })
            }
        }
    })()

    try {
        while (true) {
            const item = await nextItem()
            if (item.done) {
                break
            }

            yield item.value!
        }
    } finally {
        streamClosed = true
        reader.cancel().catch(() => {})
    }

    await pump
}
