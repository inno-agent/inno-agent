import { ThreadPrimitive, AuiIf } from '@assistant-ui/react'
import type { FC } from 'react'
import { Route } from '../../../../projects/app/routes/index'
import { Composer } from '@libs/chat/ui/composer/Composer'
import { MyRuntimeProvider } from '@libs/chat/runtime/MyRuntimeProvider'
import { ThreadMessage } from '@libs/chat/ui/messages/ThreadMessage'
import { ThreadScrollToBottom } from '@libs/chat/ui/thread/ThreadScrollToBottom'
import { ThreadWelcome } from '@libs/chat/ui/thread/ThreadWelcome'

export const Thread: FC = () => {
    const { chatId } = Route.useSearch()

    return (
        <MyRuntimeProvider key={chatId ?? 'new'} initialChatId={chatId}>
            <ThreadPrimitive.Root
                className="aui-root aui-thread-root bg-background @container flex h-full flex-col"
                style={{
                    ['--thread-max-width' as string]: '44rem',
                    ['--composer-radius' as string]: '24px',
                    ['--composer-padding' as string]: '10px',
                }}
            >
                <ThreadPrimitive.Viewport
                    turnAnchor="top"
                    data-slot="aui_thread-viewport"
                    className="relative flex flex-1 flex-col overflow-x-auto overflow-y-scroll scroll-smooth"
                >
                    <div className="mx-auto flex w-full max-w-(--thread-max-width) flex-1 flex-col px-4 pt-4">
                        <AuiIf condition={(state) => state.thread.isEmpty}>
                            <ThreadWelcome />
                        </AuiIf>

                        <div
                            data-slot="aui_message-group"
                            className="mb-10 flex flex-col gap-y-8 empty:hidden"
                        >
                            <ThreadPrimitive.Messages>
                                {() => <ThreadMessage />}
                            </ThreadPrimitive.Messages>
                        </div>

                        <ThreadPrimitive.ViewportFooter className="aui-thread-viewport-footer bg-background sticky bottom-0 mt-auto flex flex-col gap-4 overflow-visible rounded-t-(--composer-radius) pb-4 md:pb-6">
                            <ThreadScrollToBottom />
                            <Composer />
                        </ThreadPrimitive.ViewportFooter>
                    </div>
                </ThreadPrimitive.Viewport>
            </ThreadPrimitive.Root>
        </MyRuntimeProvider>
    )
}
