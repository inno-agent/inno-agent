import { AuiProvider, Suggestions, useAui } from '@assistant-ui/react'
import { Thread } from '@libs/chat/ui/thread'
import { ModelContext } from '@libs/chat/runtime/ModelContext'
import { useSelectedModel } from '@libs/chat/runtime/useSelectedModel'

const suggestions = Suggestions([
    {
        title: 'Привет!',
        label: 'начать диалог',
        prompt: 'Привет! Чем ты можешь помочь?',
    },
    {
        title: 'Разбери идею',
        label: 'структурировать задачу',
        prompt: 'Помоги структурировать задачу и предложи следующий шаг.',
    },
])

export const ChatWindow = () => {
    const aui = useAui({ suggestions })
    const modelState = useSelectedModel()

    return (
        <ModelContext.Provider value={modelState}>
            <AuiProvider value={aui}>
                <Thread />
            </AuiProvider>
        </ModelContext.Provider>
    )
}
