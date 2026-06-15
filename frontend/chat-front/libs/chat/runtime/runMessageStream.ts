import type { ThreadMessageLike } from '@assistant-ui/react'
import type { Dispatch, MutableRefObject, SetStateAction } from 'react'
import { streamMessage } from '@libs/chat/api/chatApi'
import {
    handleChunkEvent,
    handleDoneEvent,
    handleErrorEvent,
    handleStatusEvent,
} from './streamEventHandlers'

interface RunMessageStreamParams {
    initialChatId?: string
    prompt: string
    model?: string
    chatIdRef: MutableRefObject<string>
    pendingNavigationChatIdRef: MutableRefObject<string | null>
    setMessages: Dispatch<SetStateAction<readonly ThreadMessageLike[]>>
    navigateToChat: (chatId: string) => void | Promise<void>
}

export async function runMessageStream({
    initialChatId,
    prompt,
    model,
    chatIdRef,
    pendingNavigationChatIdRef,
    setMessages,
    navigateToChat,
}: RunMessageStreamParams) {
    const stream = await streamMessage(chatIdRef.current, prompt, model)
    let textContent = ''

    for await (const event of stream) {
        switch (event.type) {
            case 'status':
                handleStatusEvent(event, {
                    initialChatId,
                    chatIdRef,
                    pendingNavigationChatIdRef,
                    setMessages,
                    navigateToChat,
                })
                break
            case 'chunk':
                textContent = handleChunkEvent(event, textContent, {
                    initialChatId,
                    chatIdRef,
                    pendingNavigationChatIdRef,
                    setMessages,
                    navigateToChat,
                })
                break
            case 'done':
                await handleDoneEvent({
                    initialChatId,
                    chatIdRef,
                    pendingNavigationChatIdRef,
                    setMessages,
                    navigateToChat,
                })
                break
            case 'error':
                handleErrorEvent(event)
                break
        }
    }
}
