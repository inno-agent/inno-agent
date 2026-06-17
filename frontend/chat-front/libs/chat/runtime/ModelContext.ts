import { createContext, useContext } from 'react'
import type { ModelOption } from '@libs/chat/model/availableModels'

export interface ModelContextValue {
    models: ModelOption[]
    selectedModelId: string
    setSelectedModelId: (modelId: string) => void
}

export const ModelContext = createContext<ModelContextValue | null>(null)

export function useModelContext(): ModelContextValue {
    const ctx = useContext(ModelContext)
    if (!ctx) {
        throw new Error('useModelContext must be used within ModelProvider')
    }
    return ctx
}
