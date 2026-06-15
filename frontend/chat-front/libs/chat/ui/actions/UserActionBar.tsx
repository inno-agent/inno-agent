import { ActionBarPrimitive } from '@assistant-ui/react'
import { PencilIcon } from 'lucide-react'
import type { FC } from 'react'
import { TooltipIconButton } from '@libs/chat/ui/common/TooltipIconButton'

export const UserActionBar: FC = () => {
    return (
        <ActionBarPrimitive.Root
            hideWhenRunning
            autohide="not-last"
            className="aui-user-action-bar-root flex flex-col items-end"
        >
            <ActionBarPrimitive.Edit asChild>
                <TooltipIconButton tooltip="Edit" className="aui-user-action-edit p-4">
                    <PencilIcon />
                </TooltipIconButton>
            </ActionBarPrimitive.Edit>
        </ActionBarPrimitive.Root>
    )
}
