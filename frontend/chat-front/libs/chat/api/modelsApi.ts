import type { ModelOption } from '@libs/chat/model/availableModels'

interface CatalogModel {
    id: string
    name: string
    description: string
}

interface CatalogResponse {
    models: CatalogModel[]
    default: string
}

export interface ModelCatalog {
    models: ModelOption[]
    defaultId: string
}

export const listModels = async (): Promise<ModelCatalog> => {
    const token = localStorage.getItem('aicore_token')
    const response = await fetch('/llm/v1/models', {
        headers: {
            Accept: 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
    })
    if (!response.ok) throw new Error('Failed to load models')
    const data: CatalogResponse = await response.json()
    return {
        models: data.models.map((m) => ({
            id: m.id,
            label: m.name,
            description: m.description,
        })),
        defaultId: data.default,
    }
}
