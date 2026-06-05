import { createParser, type EventSourceMessage } from 'eventsource-parser'
import type { ChatStreamEvent } from '@libs/chat/model/types'

export async function* parseSseStream(
    response: Response,
): AsyncGenerator<ChatStreamEvent, void, unknown> {
    const reader = response.body?.getReader()
    if (!reader) throw new Error('No reader available')

    const decoder = new TextDecoder()
    const events: ChatStreamEvent[] = []
    let done = false

    const parser = createParser({
        onEvent: (event: EventSourceMessage) => {
            if (event.data === '[DONE]') {
                done = true
                return
            }

            try {
                events.push(JSON.parse(event.data) as ChatStreamEvent)
            } catch {
                // Ignore malformed SSE payloads and keep consuming the stream.
            }
        },
    })

    while (!done) {
        const result = await reader.read()
        if (result.done) break

        parser.feed(decoder.decode(result.value, { stream: true }))

        while (events.length > 0) {
            yield events.shift()!
        }
    }

    while (events.length > 0) {
        yield events.shift()!
    }
}
