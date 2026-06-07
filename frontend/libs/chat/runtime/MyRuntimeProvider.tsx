import type { AppendMessage, ThreadMessageLike } from '@assistant-ui/react'
import {
    AssistantRuntimeProvider,
    useExternalStoreRuntime,
} from '@assistant-ui/react'
import { useCallback, useRef, useState } from 'react'
import { streamMessage } from '@libs/chat/api/chatApi'
import {
    appendAssistantError,
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
    const [messages, setMessages] = useState<readonly ThreadMessageLike[]>([])
    const [isRunning, setIsRunning] = useState<boolean>(false)
    const chatIdRef = useRef<string>(initialChatId ?? 'new')

    const onNew = useCallback(
        async (message: AppendMessage) => {
            const firstPart = message.content[0]
            if (firstPart?.type !== 'text') {
                throw new Error('Only text content is supported')
            }

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
                            }
                            break
                        case 'chunk':
                            textContent += event.content
                            setMessages((prev) => upsertAssistantText(prev, textContent))
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
        [],
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
