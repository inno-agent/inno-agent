import type { ThreadMessageLike } from '@assistant-ui/react'
import type { ChatRequestMessage, MessageContentPart } from './types'

export const createUserTextMessage = (text: string): ThreadMessageLike => ({
    role: 'user',
    content: [{ type: 'text', text }],
})

export const toChatRequestMessages = (
    messages: readonly ThreadMessageLike[],
): ChatRequestMessage[] =>
    messages.map((message) => ({
        role: message.role,
        content:
            typeof message.content === 'string'
                ? message.content
                : ((message.content[0] as { text?: string })?.text ?? ''),
    }))

const updateAssistantContent = (
    messages: readonly ThreadMessageLike[],
    updater: (content: MessageContentPart[]) => MessageContentPart[],
): ThreadMessageLike[] => {
    const nextMessages = [...messages]
    const lastMessage = nextMessages[nextMessages.length - 1]

    if (lastMessage?.role === 'assistant') {
        const content = Array.isArray(lastMessage.content)
            ? [...lastMessage.content]
            : [{ type: 'text' as const, text: lastMessage.content as string }]

        nextMessages[nextMessages.length - 1] = {
            ...lastMessage,
            content: updater(content),
        }

        return nextMessages
    }

    nextMessages.push({ role: 'assistant', content: updater([]) })
    return nextMessages
}

export const upsertAssistantText = (
    messages: readonly ThreadMessageLike[],
    text: string,
): ThreadMessageLike[] =>
    updateAssistantContent(messages, (content) => {
        const index = content.findIndex((part) => part.type === 'text')
        const textPart = { type: 'text' as const, text }

        if (index >= 0) {
            content[index] = textPart
        } else {
            content.push(textPart)
        }

        return content
    })


export const appendAssistantError = (
    messages: readonly ThreadMessageLike[],
): ThreadMessageLike[] => [
    ...messages,
    {
        role: 'assistant',
        content: [
            {
                type: 'text',
                text: 'Sorry, an error occurred. Please try again.',
            },
        ],
    },
]
