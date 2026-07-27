import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { requestReview } from '@/api/review'
import { apiClient } from '@/api/client'

vi.mock('@/api/client', () => ({
    apiClient: {
        post: vi.fn(),
    },
}))

describe('requestReview', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('converts ReviewRequest with only prId to payload with pr_id', async () => {
        const mockResponse = { data: { review_markdown: '# Review' } }
        vi.mocked(apiClient.post).mockResolvedValue(mockResponse)

        await requestReview({ prId: '123' })

        expect(apiClient.post).toHaveBeenCalledWith('/review', { pr_id: '123' })
    })

    it('includes diff in payload when present', async () => {
        const mockResponse = { data: { review_markdown: '# Review' } }
        vi.mocked(apiClient.post).mockResolvedValue(mockResponse)

        await requestReview({ prId: '123', diff: 'diff content' })

        expect(apiClient.post).toHaveBeenCalledWith('/review', {
            pr_id: '123',
            diff: 'diff content',
        })
    })

    it('includes model in payload when present', async () => {
        const mockResponse = { data: { review_markdown: '# Review' } }
        vi.mocked(apiClient.post).mockResolvedValue(mockResponse)

        await requestReview({ prId: '123', model: 'qwen2.5-coder:1.5b' })

        expect(apiClient.post).toHaveBeenCalledWith('/review', {
            pr_id: '123',
            model: 'qwen2.5-coder:1.5b',
        })
    })

    it('includes all fields when all are provided', async () => {
        const mockResponse = { data: { review_markdown: '# Review' } }
        vi.mocked(apiClient.post).mockResolvedValue(mockResponse)

        await requestReview({
            prId: '123',
            diff: 'diff content',
            model: 'llama3.2:1b',
        })

        expect(apiClient.post).toHaveBeenCalledWith('/review', {
            pr_id: '123',
            diff: 'diff content',
            model: 'llama3.2:1b',
        })
    })

    it('extracts review_markdown from ReviewResponse', async () => {
        const mockResponse = { data: { review_markdown: '# Good work!' } }
        vi.mocked(apiClient.post).mockResolvedValue(mockResponse)

        const result = await requestReview({ prId: '123' })

        expect(result).toBe('# Good work!')
    })

    it('calls POST /review endpoint', async () => {
        const mockResponse = { data: { review_markdown: '# Review' } }
        vi.mocked(apiClient.post).mockResolvedValue(mockResponse)

        await requestReview({ prId: '123' })

        expect(apiClient.post).toHaveBeenCalledWith('/review', expect.any(Object))
        expect(apiClient.post).toHaveBeenCalledTimes(1)
    })

    it('throws error when request fails', async () => {
        const mockError = new Error('Network error')
        vi.mocked(apiClient.post).mockRejectedValue(mockError)

        await expect(requestReview({ prId: '123' })).rejects.toThrow('Network error')
    })
})
