import {
    ActionBarMorePrimitive,
    ActionBarPrimitive,
    AuiIf,
} from '@assistant-ui/react'
import {
    CheckIcon,
    CopyIcon,
    DownloadIcon,
    MoreHorizontalIcon,
    RefreshCwIcon,
} from 'lucide-react'
import type { FC } from 'react'
import { TooltipIconButton } from '@libs/chat/ui/common/TooltipIconButton'

export const AssistantActionBar: FC = () => {
    return (
        <ActionBarPrimitive.Root
            hideWhenRunning
            autohide="not-last"
            className="aui-assistant-action-bar-root text-muted-foreground col-start-3 row-start-2 -ms-1 flex gap-1"
        >
            <ActionBarPrimitive.Copy asChild>
                <TooltipIconButton tooltip="Copy">
                    <AuiIf condition={(state) => state.message.isCopied}>
                        <CheckIcon />
                    </AuiIf>
                    <AuiIf condition={(state) => !state.message.isCopied}>
                        <CopyIcon />
                    </AuiIf>
                </TooltipIconButton>
            </ActionBarPrimitive.Copy>
            <ActionBarPrimitive.Reload asChild>
                <TooltipIconButton tooltip="Refresh">
                    <RefreshCwIcon />
                </TooltipIconButton>
            </ActionBarPrimitive.Reload>
            <ActionBarMorePrimitive.Root>
                <ActionBarMorePrimitive.Trigger asChild>
                    <TooltipIconButton
                        tooltip="More"
                        className="data-[state=open]:bg-accent"
                    >
                        <MoreHorizontalIcon />
                    </TooltipIconButton>
                </ActionBarMorePrimitive.Trigger>
                <ActionBarMorePrimitive.Content
                    side="bottom"
                    align="start"
                    className="aui-action-bar-more-content bg-popover text-popover-foreground z-50 min-w-32 overflow-hidden rounded-md border p-1 shadow-md"
                >
                    <ActionBarPrimitive.ExportMarkdown asChild>
                        <ActionBarMorePrimitive.Item className="aui-action-bar-more-item hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none select-none">
                            <DownloadIcon className="size-4" />
                            Export as Markdown
                        </ActionBarMorePrimitive.Item>
                    </ActionBarPrimitive.ExportMarkdown>
                </ActionBarMorePrimitive.Content>
            </ActionBarMorePrimitive.Root>
        </ActionBarPrimitive.Root>
    )
}
