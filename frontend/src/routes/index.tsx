import { createFileRoute } from '@tanstack/react-router'
import { ChatWindow } from '../widgets/ChatWindow'

export const Route = createFileRoute('/')({
    component: ChatWindow,
})
