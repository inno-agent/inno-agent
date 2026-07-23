import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { listModels } from '@/api/models'
import { llmClient } from '@/api/client'

vi.mock('@/api/client', () => ({
    llmClient: {
        get: vi.fn(),
    },
}))

describe('listModels', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('maps CatalogResponse to ModelCatalog structure', async () => {
        const mockResponse = {
            data: {
                models: [
                    { id: 'auto', name: 'Auto', description: 'Automatically selects the best model' },
                    { id: 'qwen2.5:0.5b', name: 'Fast', description: 'Tiny model, fastest responses' },
                ],
                default: 'auto',
            },
        }
        vi.mocked(llmClient.get).mockResolvedValue(mockResponse)

        const result = await listModels()

        expect(result.models).toHaveLength(2)
        expect(result.models[0]).toEqual({
            id: 'auto',
            label: 'Auto',
            description: 'Automatically selects the best model',
        })
        expect(result.models[1]).toEqual({
            id: 'qwen2.5:0.5b',
            label: 'Fast',
            description: 'Tiny model, fastest responses',
        })
    })

    it('extracts defaultId from CatalogResponse.default', async () => {
        const mockResponse = {
            data: {
                models: [
                    { id: 'auto', name: 'Auto', description: 'Auto mode' },
                    { id: 'qwen2.5-coder:1.5b', name: 'Code', description: 'Coding model' },
                ],
                default: 'auto',
            },
        }
        vi.mocked(llmClient.get).mockResolvedValue(mockResponse)

        const result = await listModels()

        expect(result.defaultId).toBe('auto')
    })

    it('returns ModelCatalog with models and defaultId', async () => {
        const mockResponse = {
            data: {
                models: [
                    { id: 'llama3.2:1b', name: 'General', description: 'General Q&A' },
                ],
                default: 'llama3.2:1b',
            },
        }
        vi.mocked(llmClient.get).mockResolvedValue(mockResponse)

        const result = await listModels()

        expect(result).toHaveProperty('models')
        expect(result).toHaveProperty('defaultId')
        expect(Array.isArray(result.models)).toBe(true)
        expect(typeof result.defaultId).toBe('string')
    })

    it('calls GET /models endpoint', async () => {
        const mockResponse = {
            data: {
                models: [],
                default: 'auto',
            },
        }
        vi.mocked(llmClient.get).mockResolvedValue(mockResponse)

        await listModels()

        expect(llmClient.get).toHaveBeenCalledWith('/models')
        expect(llmClient.get).toHaveBeenCalledTimes(1)
    })

    it('handles empty models list', async () => {
        const mockResponse = {
            data: {
                models: [],
                default: 'auto',
            },
        }
        vi.mocked(llmClient.get).mockResolvedValue(mockResponse)

        const result = await listModels()

        expect(result.models).toEqual([])
        expect(result.defaultId).toBe('auto')
    })

    it('throws error when request fails', async () => {
        const mockError = new Error('Network error')
        vi.mocked(llmClient.get).mockRejectedValue(mockError)

        await expect(listModels()).rejects.toThrow('Network error')
    })

    it('maps id to id, name to label, description to description', async () => {
        const mockResponse = {
            data: {
                models: [
                    { id: 'qwen2.5-coder:1.5b', name: 'Code', description: 'Programming model' },
                ],
                default: 'qwen2.5-coder:1.5b',
            },
        }
        vi.mocked(llmClient.get).mockResolvedValue(mockResponse)

        const result = await listModels()

        expect(result.models[0].id).toBe('qwen2.5-coder:1.5b')
        expect(result.models[0].label).toBe('Code')
        expect(result.models[0].description).toBe('Programming model')
    })
})
