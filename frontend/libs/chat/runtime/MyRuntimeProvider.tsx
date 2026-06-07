import type { AppendMessage, ThreadMessageLike } from '@assistant-ui/react'
import {
    AssistantRuntimeProvider,
    useExternalStoreRuntime,
} from '@assistant-ui/react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { useNavigate } from '@tanstack/react-router'
import { getChatHistory, notifyChatsUpdated, streamMessage } from '@libs/chat/api/chatApi'
import {
    appendAssistantError,
    fromApiMessage,
    createUserTextMessage,
    upsertAssistantText,
} from '@libs/chat/model/messageMappers'

export function MyRuntimeProvider({
    children,
    initialChatId,
}: Readonly<{
    children: React.ReactNode
    initialChatId?: string
}>) {
    const navigate = useNavigate({ from: '/' })
    const [messages, setMessages] = useState<readonly ThreadMessageLike[]>([])
    const [isRunning, setIsRunning] = useState<boolean>(false)
    const chatIdRef = useRef<string>(initialChatId ?? 'new')
    const pendingNavigationChatIdRef = useRef<string | null>(null)

    useEffect(() => {
        let isMounted = true

        chatIdRef.current = initialChatId ?? 'new'
        pendingNavigationChatIdRef.current = null
        // eslint-disable-next-line react-hooks/set-state-in-effect
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
    }, [initialChatId])

    const onNew = useCallback(
        async (message: AppendMessage) => {
            const firstPart = message.content[0]
            if (firstPart?.type !== 'text') {
                throw new Error('Only text content is supported')
            }

            pendingNavigationChatIdRef.current = null
            setMessages((prev) => [...prev, createUserTextMessage(firstPart.text)])
            setIsRunning(true)

            try {
                const stream = await streamMessage(chatIdRef.current, firstPart.text)
                let textContent = ''

                for await (const event of stream) {
                    switch (event.type) {
                        case 'status':
                            if ('chat_id' in event && event.chat_id) {
                                chatIdRef.current = event.chat_id
                                if (!initialChatId) {
                                    pendingNavigationChatIdRef.current = event.chat_id
                                }
                            }
                            break
                        case 'chunk':
                            textContent += event.content
                            flushSync(() => {
                                setMessages((prev) => upsertAssistantText(prev, textContent))
                            })
                            break
                        case 'done':
                            if (pendingNavigationChatIdRef.current) {
                                void navigate({
                                    to: '/',
                                    search: { chatId: pendingNavigationChatIdRef.current },
                                    replace: true,
                                })
                                pendingNavigationChatIdRef.current = null
                            }
                            notifyChatsUpdated(chatIdRef.current)
                            break
                        case 'error':
                            throw new Error(event.message)
                    }
                }
            } catch (error) {
                console.error('Error:', error)
                setMessages((prev) => appendAssistantError(prev))
            } finally {
                setIsRunning(false)
            }
        },
        [initialChatId, navigate],
    )

    const runtime = useExternalStoreRuntime<ThreadMessageLike>({
        messages,
        setMessages,
        onNew,
        convertMessage: (message) => message,
        isRunning,
    })

    return (
        <AssistantRuntimeProvider runtime={runtime}>
            {children}
        </AssistantRuntimeProvider>
    )
}
