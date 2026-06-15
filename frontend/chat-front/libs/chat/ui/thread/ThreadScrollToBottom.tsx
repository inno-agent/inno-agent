import { ThreadPrimitive } from '@assistant-ui/react'
import { ArrowDownIcon } from 'lucide-react'
import type { FC } from 'react'
import { TooltipIconButton } from '@libs/chat/ui/common/TooltipIconButton'

export const ThreadScrollToBottom: FC = () => {
    return (
        <ThreadPrimitive.ScrollToBottom asChild>
            <TooltipIconButton
                tooltip="Scroll to bottom"
                variant="outline"
                className="aui-thread-scroll-to-bottom dark:border-border dark:bg-background dark:hover:bg-accent absolute -top-12 z-10 self-center rounded-full p-4 disabled:invisible"
            >
                <ArrowDownIcon />
            </TooltipIconButton>
        </ThreadPrimitive.ScrollToBottom>
    )
}
