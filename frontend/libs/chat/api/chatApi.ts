import { parseSseStream } from '@libs/chat/lib/parseSseStream'
import type { ChatItem, Message } from '@libs/chat/model/types'

export const listChats = async (userId: string, limit?: number, offset?: number) => {
    const params = new URLSearchParams({ user_id: userId })
    if (limit !== undefined) params.append('limit', String(limit))
    if (offset !== undefined) params.append('offset', String(offset))
    const url = `/api/v1/chats?${params}`
    const response = await fetch(url)
    if (!response.ok) throw new Error('Failed to fetch chats')
    const data = await response.json()
    return data.chats as ChatItem[]
}

export const getChatHistory = async (chatId: string, userId: string, limit?: number, offset?: number) => {
    const params = new URLSearchParams({ user_id: userId })
    if (limit !== undefined) params.append('limit', String(limit))
    if (offset !== undefined) params.append('offset', String(offset))
    const url = `/api/v1/chats/${chatId}/messages?${params}`
    const response = await fetch(url)
    if (!response.ok) throw new Error('Failed to fetch chat history')
    const data = await response.json()
    return data as { chat_id: string; messages: Message[]; total: number }
}

export const streamMessage = async (chatId: string, userId: string, message: string, temperature?: number, maxTokens?: number) => {
    const params = new URLSearchParams({ user_id: userId, message })
    if (temperature !== undefined) params.append('temperature', String(temperature))
    if (maxTokens !== undefined) params.append('max_tokens', String(maxTokens))
    const url = `/api/v1/chats/${chatId}/stream?${params}`
    const response = await fetch(url)
    if (!response.ok) throw new Error('Failed to stream message')
    return parseSseStream(response)
}

