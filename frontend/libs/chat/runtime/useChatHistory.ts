import type { ThreadMessageLike } from '@assistant-ui/react'
import type { Dispatch, MutableRefObject, SetStateAction } from 'react'
import { useEffect } from 'react'
import { getChatHistory } from '@libs/chat/api/chatApi'
import { fromApiMessage } from '@libs/chat/model/messageMappers'

interface UseChatHistoryParams {
    initialChatId?: string
    setMessages: Dispatch<SetStateAction<readonly ThreadMessageLike[]>>
    chatIdRef: MutableRefObject<string>
    pendingNavigationChatIdRef: MutableRefObject<string | null>
}

export function useChatHistory({
    initialChatId,
    setMessages,
    chatIdRef,
    pendingNavigationChatIdRef,
}: UseChatHistoryParams) {
    useEffect(() => {
        let isMounted = true

        chatIdRef.current = initialChatId ?? 'new'
        pendingNavigationChatIdRef.current = null
        setMessages([])

        if (!initialChatId) {
            return () => {
                isMounted = false
            }
        }

        const loadChatHistory = async () => {
            try {
                const { messages: historyMessages } = await getChatHistory(initialChatId)

                if (!isMounted) {
                    return
                }

                setMessages(historyMessages.map(fromApiMessage))
            } catch (error) {
                if (!isMounted) {
                    return
                }

                console.error('Failed to load chat history', error)
                setMessages([])
            }
        }

        void loadChatHistory()

        return () => {
            isMounted = false
        }
    }, [chatIdRef, initialChatId, pendingNavigationChatIdRef, setMessages])
}
