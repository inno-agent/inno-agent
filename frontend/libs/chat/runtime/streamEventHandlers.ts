import type { ThreadMessageLike } from '@assistant-ui/react'
import type { Dispatch, MutableRefObject, SetStateAction } from 'react'
import { flushSync } from 'react-dom'
import { notifyChatsUpdated } from '@libs/chat/api/chatApi'
import { upsertAssistantText } from '@libs/chat/model/messageMappers'
import type { ChatStreamEvent } from '@libs/chat/model/types'

interface StreamHandlerContext {
    initialChatId?: string
    chatIdRef: MutableRefObject<string>
    pendingNavigationChatIdRef: MutableRefObject<string | null>
    setMessages: Dispatch<SetStateAction<readonly ThreadMessageLike[]>>
    navigateToChat: (chatId: string) => void | Promise<void>
}

export const handleStatusEvent = (
    event: Extract<ChatStreamEvent, { type: 'status' }>,
    { initialChatId, chatIdRef, pendingNavigationChatIdRef }: StreamHandlerContext,
) => {
    if (!event.chat_id) {
        return
    }

    chatIdRef.current = event.chat_id
    if (!initialChatId) {
        pendingNavigationChatIdRef.current = event.chat_id
    }
}

export const handleChunkEvent = (
    event: Extract<ChatStreamEvent, { type: 'chunk' }>,
    textContent: string,
    { setMessages }: StreamHandlerContext,
) => {
    const nextTextContent = textContent + event.content

    flushSync(() => {
        setMessages((prev) => upsertAssistantText(prev, nextTextContent))
    })

    console.log('[chat:sse] chunk committed', {
        at: new Date().toISOString(),
        chunkLength: event.content.length,
        totalLength: nextTextContent.length,
    })

    return nextTextContent
}

export const handleDoneEvent = async ({
    chatIdRef,
    pendingNavigationChatIdRef,
    navigateToChat,
}: StreamHandlerContext) => {
    if (pendingNavigationChatIdRef.current) {
        await navigateToChat(pendingNavigationChatIdRef.current)
        pendingNavigationChatIdRef.current = null
    }

    notifyChatsUpdated(chatIdRef.current)
}

export const handleErrorEvent = (event: Extract<ChatStreamEvent, { type: 'error' }>) => {
    throw new Error(event.message)
}
