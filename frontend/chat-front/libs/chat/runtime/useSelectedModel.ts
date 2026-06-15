import { useCallback, useState } from 'react'
import { AVAILABLE_MODELS, DEFAULT_MODEL_ID } from '@libs/chat/model/availableModels'

const STORAGE_KEY = 'selected_model_id'

export function useSelectedModel() {
    const [selectedModelId, setSelectedModelIdState] = useState<string>(() => {
        const stored = localStorage.getItem(STORAGE_KEY)
        if (stored && AVAILABLE_MODELS.some((m) => m.id === stored)) {
            return stored
        }
        return DEFAULT_MODEL_ID
    })

    const setSelectedModelId = useCallback((modelId: string) => {
        setSelectedModelIdState(modelId)
        localStorage.setItem(STORAGE_KEY, modelId)
    }, [])

    return {
        selectedModelId,
        setSelectedModelId,
    }
}
