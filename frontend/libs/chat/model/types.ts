import type { ThreadMessageLike } from '@assistant-ui/react'

export type ChatStreamEvent =
    | { type: 'text'; content: string }
    | { type: 'tool_call'; id: string; name: string; arguments: string }
    | { type: 'tool_result'; id: string; result: string }

export interface ChatRequestMessage {
    role: ThreadMessageLike['role']
    content: string
}

export type MessageContentPart = Exclude<ThreadMessageLike['content'], string>[number]
