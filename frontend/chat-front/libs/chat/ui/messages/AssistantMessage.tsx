import { MessagePrimitive, useAuiState } from '@assistant-ui/react'
import type { FC } from 'react'
import { AssistantActionBar } from '@libs/chat/ui/actions/AssistantActionBar'
import { BranchPicker } from '@libs/chat/ui/actions/BranchPicker'
import { AssistantMessageParts } from '@libs/chat/ui/parts/AssistantMessageParts'
import { cn } from '@shared/lib/utils'
import { MessageError } from './MessageError'
import { ThinkingIndicator } from './ThinkingIndicator'

export const AssistantMessage: FC = () => {
    const actionBarHeight = '-mb-7.5 min-h-7.5 pt-1.5'

    const isRunning = useAuiState((state) => state.thread.isRunning)
    const hasText = useAuiState((state) =>
        state.message.content.some(
            (part) => part.type === 'text' && part.text.length > 0,
        ),
    )
    const showThinking = isRunning && !hasText

    return (
        <MessagePrimitive.Root
            data-slot="aui_assistant-message-root"
            data-role="assistant"
            className="fade-in slide-in-from-bottom-1 animate-in relative duration-150"
        >
            <div
                data-slot="aui_assistant-message-content"
                className="text-foreground px-2 leading-relaxed wrap-break-word [contain-intrinsic-size:auto_24px] [content-visibility:auto]"
            >
                {showThinking ? (
                    <ThinkingIndicator />
                ) : (
                    <>
                        <AssistantMessageParts />
                        <MessageError />
                    </>
                )}
            </div>

            <div
                data-slot="aui_assistant-message-footer"
                className={cn('ms-2 flex items-center', actionBarHeight)}
            >
                <BranchPicker />
                <AssistantActionBar />
            </div>
        </MessagePrimitive.Root>
    )
}
