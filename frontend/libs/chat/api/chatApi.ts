import { parseSseStream } from '@libs/chat/lib/parseSseStream'
import type { ChatRequestMessage } from '@libs/chat/model/types'

const CHAT_ENDPOINT = '/api/chat'

export const streamChatResponse = async (messages: ChatRequestMessage[]) => {
    const response = await fetch(CHAT_ENDPOINT, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ messages }),
    })

    if (!response.ok) throw new Error('Failed to fetch response')

    return parseSseStream(response)
}
