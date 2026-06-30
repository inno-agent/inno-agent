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
import { useModelContext } from './ModelContext'

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
    const abortControllerRef = useRef<AbortController | null>(null)
    const { selectedModelId } = useModelContext()

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

            abortControllerRef.current = new AbortController()
            pendingNavigationChatIdRef.current = null
            setMessages((prev) => [
                ...prev,
                createUserTextMessage(firstPart.text),
                { role: 'assistant', content: [] },
            ])

            setIsRunning(true)

            try {
                await runMessageStream({
                    initialChatId,
                    prompt: firstPart.text,
                    model: selectedModelId,
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
                    signal: abortControllerRef.current.signal,
                })
            } catch (error) {
                if (error instanceof DOMException && error.name === 'AbortError') {
                    console.log('Message generation cancelled')
                } else if (error instanceof Error && error.name === 'AbortError') {
                    console.log('Message generation cancelled')
                } else {
                    console.error('Error:', error)
                    setMessages((prev) => appendAssistantError(prev))
                }
            } finally {
                setIsRunning(false)
                abortControllerRef.current = null
            }
        },
        [initialChatId, navigate, selectedModelId],
    )

    const onCancel = useCallback(async () => {
        if (abortControllerRef.current) {
            abortControllerRef.current.abort()
            abortControllerRef.current = null
        }
        setIsRunning(false)
    }, [])

    const runtime = useExternalStoreRuntime<ThreadMessageLike>({
        messages,
        setMessages,
        onNew,
        onCancel,
        convertMessage: (message) => message,
        isRunning,
    })

    return (
        <AssistantRuntimeProvider runtime={runtime}>
            {children}
        </AssistantRuntimeProvider>
    )
}
