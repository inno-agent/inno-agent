import { parseSseStream } from '@libs/chat/lib/parseSseStream'
import type { ChatItem, Message } from '@libs/chat/model/types'
import { apiClient, apiEndpoints, buildApiUrl } from '@shared/api/axios'

export const listChats = async (userId: string, limit?: number, offset?: number) => {
    const { data } = await apiClient.get<{ chats: ChatItem[] }>(apiEndpoints.chats, {
        params: {
            user_id: userId,
            limit,
            offset,
        },
    })

    return data.chats
}

export const getChatHistory = async (chatId: string, userId: string, limit?: number, offset?: number) => {
    const { data } = await apiClient.get<{ chat_id: string; messages: Message[]; total: number }>(
        apiEndpoints.chatMessages(chatId),
        {
            params: {
                user_id: userId,
                limit,
                offset,
            },
        },
    )

    return data
}

export const streamMessage = async (chatId: string, userId: string, message: string, temperature?: number, maxTokens?: number) => {
    const url = buildApiUrl(apiEndpoints.chatStream(chatId), {
        user_id: userId,
        message,
        temperature,
        max_tokens: maxTokens,
    })

    const response = await fetch(url, {
        credentials: apiClient.defaults.withCredentials ? 'include' : 'same-origin',
        headers: {
            Accept: 'text/event-stream',
        },
    })

    if (!response.ok) throw new Error('Failed to stream message')
    return parseSseStream(response)
}
