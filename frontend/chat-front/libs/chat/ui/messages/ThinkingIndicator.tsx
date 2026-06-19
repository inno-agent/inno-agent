import type { FC } from 'react'

export const ThinkingIndicator: FC = () => (
    <span
        data-slot="aui_thinking-indicator"
        className="text-muted-foreground flex items-center gap-1.5 text-sm"
        aria-live="polite"
    >
        <span className="animate-pulse">●</span>
        Processing…
    </span>
)