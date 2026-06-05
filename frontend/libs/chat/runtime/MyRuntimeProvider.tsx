import type { AppendMessage, ThreadMessageLike } from '@assistant-ui/react'
import {
    AssistantRuntimeProvider,
    useExternalStoreRuntime,
} from '@assistant-ui/react'
import { useCallback, useState } from 'react'
import { streamMessage } from '@libs/chat/api/chatApi'
import {
    appendAssistantError,
    createUserTextMessage,
    upsertAssistantText,
} from '@libs/chat/model/messageMappers'

export function MyRuntimeProvider({
     children,
     chatId,
     userId,
     }: Readonly<{
    children: React.ReactNode
    chatId: string
    userId: string
}>) {
    const [messages, setMessages] = useState<readonly ThreadMessageLike[]>([])
    const [isRunning, setIsRunning] = useState(false)

    const onNew = useCallback(
        async (message: AppendMessage) => {
            if (message.content[0]?.type !== 'text') {
                throw new Error('Only text content is supported')
            }

            const nextMessages = [
                ...messages,
                createUserTextMessage(message.content[0].text),
            ]

            setMessages(nextMessages)
            setIsRunning(true)

            try {
                const stream = await streamMessage(chatId, userId, message.content[0].text)
                let textContent = ''

                for await (const event of stream) {
                    switch (event.type) {
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
        [messages],
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
