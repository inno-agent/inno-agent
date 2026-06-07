import { parseSseStream } from '@libs/chat/lib/parseSseStream'
import type { ChatItem, Message } from '@libs/chat/model/types'
import { apiClient, apiEndpoints } from '@shared/api/axios'

export const listChats = async (limit?: number, offset?: number) => {
    const { data } = await apiClient.get<{ chats: ChatItem[] }>(apiEndpoints.chats, {
        params: { limit, offset },
    })

    return data.chats
}

export const getChatHistory = async (chatId: string, limit?: number, offset?: number) => {
    const { data } = await apiClient.get<{ chat_id: string; messages: Message[]; total: number }>(
        apiEndpoints.chatMessages(chatId),
        { params: { limit, offset } },
    )

    return data
}

export const streamMessage = async (chatId: string, message: string) => {
    const token = localStorage.getItem('aicore_token')
    const response = await fetch(`/api/v1${apiEndpoints.chatStream(chatId)}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            Accept: 'text/event-stream',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ message }),
    })

    if (!response.ok) throw new Error('Failed to stream message')
    return parseSseStream(response)
}
