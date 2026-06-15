import { SuggestionPrimitive, ThreadPrimitive } from '@assistant-ui/react'
import type { FC } from 'react'
import { Button } from '@shared/ui/button'

export const ThreadWelcome: FC = () => {
    return (
        <div className="aui-thread-welcome-root my-auto flex grow flex-col">
            <div className="aui-thread-welcome-center flex w-full grow flex-col items-center justify-center">
                <div className="aui-thread-welcome-message flex size-full flex-col justify-center px-4">
                    <h1 className="aui-thread-welcome-message-inner fade-in slide-in-from-bottom-1 animate-in fill-mode-both text-2xl font-semibold duration-200">
                        Hello there!
                    </h1>
                    <p className="aui-thread-welcome-message-inner fade-in slide-in-from-bottom-1 animate-in fill-mode-both text-muted-foreground text-xl delay-75 duration-200">
                        How can I help you today?
                    </p>
                </div>
            </div>
            <ThreadSuggestions />
        </div>
    )
}

const ThreadSuggestions: FC = () => {
    return (
        <div className="aui-thread-welcome-suggestions grid w-full gap-2 pb-4 @md:grid-cols-2">
            <ThreadPrimitive.Suggestions>
                {() => <ThreadSuggestionItem />}
            </ThreadPrimitive.Suggestions>
        </div>
    )
}

const ThreadSuggestionItem: FC = () => {
    return (
        <div className="aui-thread-welcome-suggestion-display fade-in slide-in-from-bottom-2 animate-in fill-mode-both duration-200 nth-[n+3]:hidden @md:nth-[n+3]:block">
            <SuggestionPrimitive.Trigger send asChild>
                <Button
                    variant="ghost"
                    className="aui-thread-welcome-suggestion bg-background hover:bg-muted h-auto w-full flex-wrap items-start justify-start gap-1 rounded-3xl border px-4 py-3 text-start text-sm transition-colors @md:flex-col"
                >
                    <SuggestionPrimitive.Title className="aui-thread-welcome-suggestion-text-1 font-medium" />
                    <SuggestionPrimitive.Description className="aui-thread-welcome-suggestion-text-2 text-muted-foreground empty:hidden" />
                </Button>
            </SuggestionPrimitive.Trigger>
        </div>
    )
}
