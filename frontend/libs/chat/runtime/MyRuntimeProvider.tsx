import type { AppendMessage, ThreadMessageLike } from '@assistant-ui/react'
import {
    AssistantRuntimeProvider,
    useExternalStoreRuntime,
} from '@assistant-ui/react'
import type { ReactNode } from 'react'
import { useCallback, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
    appendAssistantError,
    createUserTextMessage,
} from '@libs/chat/model/messageMappers'
import { runMessageStream } from './runMessageStream'
import { useChatHistory } from './useChatHistory'

export function MyRuntimeProvider({
    children,
    initialChatId,
}: Readonly<{
    children: ReactNode
    initialChatId?: string
}>) {
    const navigate = useNavigate({ from: '/' })
    const [messages, setMessages] = useState<readonly ThreadMessageLike[]>([])
    const [isRunning, setIsRunning] = useState<boolean>(false)
    const chatIdRef = useRef<string>(initialChatId ?? 'new')
    const pendingNavigationChatIdRef = useRef<string | null>(null)

    useChatHistory({
        initialChatId,
        setMessages,
        chatIdRef,
        pendingNavigationChatIdRef,
    })

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
                await runMessageStream({
                    initialChatId,
                    prompt: firstPart.text,
                    chatIdRef,
                    pendingNavigationChatIdRef,
                    setMessages,
                    navigateToChat: async (chatId) => {
                        await navigate({
                            to: '/',
                            search: { chatId },
                            replace: true,
                        })
                    },
                })
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
