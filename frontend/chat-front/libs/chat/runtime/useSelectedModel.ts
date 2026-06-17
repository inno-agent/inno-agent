import { useCallback, useEffect, useState } from 'react'
import type { ModelOption } from '@libs/chat/model/availableModels'
import { listModels } from '@libs/chat/api/modelsApi'

const STORAGE_KEY = 'selected_model_id'

export function useSelectedModel() {
    const [models, setModels] = useState<ModelOption[]>([])
    const [selectedModelId, setSelectedModelIdState] = useState<string>(
        () => localStorage.getItem(STORAGE_KEY) ?? '',
    )

    useEffect(() => {
        let cancelled = false
        listModels()
            .then(({ models, defaultId }) => {
                if (cancelled) return
                setModels(models)
                setSelectedModelIdState((current) => {
                    if (current && models.some((m) => m.id === current)) {
                        return current
                    }
                    return defaultId
                })
            })
            .catch(() => {
                /* keep empty list; selector renders nothing until retry */
            })
        return () => {
            cancelled = true
        }
    }, [])

    const setSelectedModelId = useCallback((modelId: string) => {
        setSelectedModelIdState(modelId)
        localStorage.setItem(STORAGE_KEY, modelId)
    }, [])

    return { models, selectedModelId, setSelectedModelId }
}
