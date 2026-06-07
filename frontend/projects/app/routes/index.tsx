import { createFileRoute } from '@tanstack/react-router'
import { ChatWindow } from '@libs/chat'

export const Route = createFileRoute('/')({
    validateSearch: (search: Record<string, unknown>) => ({
        chatId: typeof search.chatId === 'string' && search.chatId.length > 0 ? search.chatId : undefined,
    }),
    component: ChatWindow,
})
