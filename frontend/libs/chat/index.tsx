import { AuiProvider, Suggestions, useAui } from '@assistant-ui/react'
import { Thread } from '@libs/chat/ui/thread'

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

    return (
        <AuiProvider value={aui}>
            <Thread />
        </AuiProvider>
    )
}
