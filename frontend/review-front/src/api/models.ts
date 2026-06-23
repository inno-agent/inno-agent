import { llmClient } from '@/api/client'

export interface ModelOption {
    id: string
    label: string
    description: string
}

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

export async function listModels(): Promise<ModelCatalog> {
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
