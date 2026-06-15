import { AuiIf, ComposerPrimitive } from '@assistant-ui/react'
import { ArrowUpIcon, SquareIcon } from 'lucide-react'
import type { FC } from 'react'
import {
    ComposerAddAttachment,
    ComposerAttachments,
} from '@libs/chat/ui/attachments'
import { TooltipIconButton } from '@libs/chat/ui/common/TooltipIconButton'
import { Button } from '@shared/ui/button'

export const Composer: FC = () => {
    return (
        <ComposerPrimitive.Root className="aui-composer-root relative flex w-full flex-col">
            <ComposerPrimitive.AttachmentDropzone asChild>
                <div
                    data-slot="aui_composer-shell"
                    className="bg-background focus-within:border-ring/75 focus-within:ring-ring/20 data-[dragging=true]:border-ring data-[dragging=true]:bg-accent/50 flex w-full flex-col gap-2 rounded-(--composer-radius) border p-(--composer-padding) transition-shadow focus-within:ring-2 data-[dragging=true]:border-dashed"
                >
                    <ComposerAttachments />
                    <ComposerPrimitive.Input
                        placeholder="Send a message..."
                        className="aui-composer-input placeholder:text-muted-foreground/80 max-h-32 min-h-10 w-full resize-none bg-transparent px-1.75 py-1 text-sm outline-none"
                        rows={1}
                        autoFocus
                        aria-label="Message input"
                    />
                    <ComposerAction />
                </div>
            </ComposerPrimitive.AttachmentDropzone>
        </ComposerPrimitive.Root>
    )
}

const ComposerAction: FC = () => {
    return (
        <div className="aui-composer-action-wrapper relative flex items-center justify-between">
            <ComposerAddAttachment />
            <AuiIf condition={(state) => !state.thread.isRunning}>
                <ComposerPrimitive.Send asChild>
                    <TooltipIconButton
                        tooltip="Send message"
                        side="bottom"
                        type="button"
                        variant="default"
                        size="icon"
                        className="aui-composer-send size-8 rounded-full"
                        aria-label="Send message"
                    >
                        <ArrowUpIcon className="aui-composer-send-icon size-4" />
                    </TooltipIconButton>
                </ComposerPrimitive.Send>
            </AuiIf>
            <AuiIf condition={(state) => state.thread.isRunning}>
                <ComposerPrimitive.Cancel asChild>
                    <Button
                        type="button"
                        variant="default"
                        size="icon"
                        className="aui-composer-cancel size-8 rounded-full"
                        aria-label="Stop generating"
                    >
                        <SquareIcon className="aui-composer-cancel-icon size-3 fill-current" />
                    </Button>
                </ComposerPrimitive.Cancel>
            </AuiIf>
        </div>
    )
}
