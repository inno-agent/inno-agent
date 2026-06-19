import { groupPartByType, MessagePrimitive } from '@assistant-ui/react'
import type { FC } from 'react'
import { MarkdownText } from '@libs/chat/ui/markdown/MarkdownText'
import {
    Reasoning,
    ReasoningContent,
    ReasoningRoot,
    ReasoningText,
    ReasoningTrigger,
} from '@libs/chat/ui/reasoning'
import { ToolFallback } from '@libs/chat/ui/tools/ToolFallback'
import {
    ToolGroupContent,
    ToolGroupRoot,
    ToolGroupTrigger,
} from '@libs/chat/ui/tools/ToolGroup'

export const AssistantMessageParts: FC = () => {
    return (
        <MessagePrimitive.GroupedParts
            groupBy={groupPartByType<
                'group-chainOfThought' | 'group-reasoning' | 'group-tool'
            >({
                reasoning: ['group-chainOfThought', 'group-reasoning'],
                'tool-call': ['group-chainOfThought', 'group-tool'],
                'standalone-tool-call': [],
            })}
        >
            {({ part, children }) => {
                switch (part.type) {
                    case 'group-chainOfThought':
                        return <div data-slot="aui_chain-of-thought">{children}</div>
                    case 'group-reasoning': {
                        const running = part.status.type === 'running'
                        return (
                            <ReasoningRoot defaultOpen={running}>
                                <ReasoningTrigger active={running} />
                                <ReasoningContent aria-busy={running}>
                                    <ReasoningText>{children}</ReasoningText>
                                </ReasoningContent>
                            </ReasoningRoot>
                        )
                    }
                    case 'group-tool':
                        return (
                            <ToolGroupRoot>
                                <ToolGroupTrigger
                                    count={part.indices.length}
                                    active={part.status.type === 'running'}
                                />
                                <ToolGroupContent>{children}</ToolGroupContent>
                            </ToolGroupRoot>
                        )
                    case 'text':
                        return <MarkdownText />
                    case 'reasoning':
                        return <Reasoning {...part} />
                    case 'tool-call':
                        return part.toolUI ?? <ToolFallback {...part} />
                    default:
                        return null
                }
            }}
        </MessagePrimitive.GroupedParts>
    )
}
