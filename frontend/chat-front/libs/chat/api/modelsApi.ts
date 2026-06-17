import type { ModelOption } from '@libs/chat/model/availableModels'
import { llmClient } from '@shared/api/axios'

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
    const { data } = await llmClient.get<CatalogResponse>('/models')
    return {
        models: data.models.map((m) => ({
            id: m.id,
            label: m.name,
            description: m.description,
        })),
        defaultId: data.default,
    }
}
