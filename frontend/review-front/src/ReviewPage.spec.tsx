import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import ReviewPage from '@/ReviewPage'


vi.mock('@/api/models', () => ({
    listModels: vi.fn().mockResolvedValue({ models: [], defaultId: '' }),
}))

vi.mock('@/api/review', () => ({
    requestReview: vi.fn(),
}))

describe('ReviewPage', () => {
    it('renders the PR review form', async () => {
        render(<ReviewPage />)

        expect(screen.getByRole('heading', { name: 'PR Reviewer' })).toBeInTheDocument()
        expect(await screen.findByLabelText('PR ID (Owner/Repo/Index)')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Generate Review' })).toBeInTheDocument()
    })
})
