import type { ThreadMessageLike } from '@assistant-ui/react'

export type ChatStreamEvent =
    | { type: 'status'; stage: string; chat_id: string }
    | { type: 'chunk'; content: string }
    | { type: 'done'; status: string; tokens_used: { prompt: number; completion: number; total: number }; finished_at: string }
    | { type: 'error'; code: string; message: string; retry_allowed: boolean }

export interface ChatRequestMessage {
    role: ThreadMessageLike['role']
    content: string
}

export interface Message {
    id: string
    role: string
    content: string
    created_at: string
}

export interface ChatItem {
    id: string
    title: string
    last_message: string
    updated_at: string
}

export type MessageContentPart = Exclude<ThreadMessageLike['content'], string>[number]
