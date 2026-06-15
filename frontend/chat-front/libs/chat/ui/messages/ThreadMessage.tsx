import { useAuiState } from '@assistant-ui/react'
import type { FC } from 'react'
import { EditComposer } from '@libs/chat/ui/composer/EditComposer'
import { AssistantMessage } from './AssistantMessage'
import { UserMessage } from './UserMessage'

export const ThreadMessage: FC = () => {
    const role = useAuiState((state) => state.message.role)
    const isEditing = useAuiState((state) => state.message.composer.isEditing)

    if (isEditing) return <EditComposer />
    if (role === 'user') return <UserMessage />
    return <AssistantMessage />
}
