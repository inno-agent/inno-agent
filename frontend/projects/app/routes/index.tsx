import { createFileRoute } from '@tanstack/react-router'
import { ChatWindow } from '@libs/chat'

export const Route = createFileRoute('/')({
    component: ChatWindow,
})
