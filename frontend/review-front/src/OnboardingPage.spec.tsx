import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import OnboardingPagе from '@/OnboardingPage'


vi.mock('@/api/consent', () => ({
    getLinkedGitFlameUsername: vi.fn().mockResolvedValue(null),
    linkGitFlameUsername: vi.fn(),
    acceptGitFlameInvite: vi.fn(),
}))

describe('OnboardingPagе', () => {
    it('renders the onboarding form', async () => {
        render(<OnboardingPagе />)

        expect(screen.getByRole('heading', { name: 'Link GitFlame account' })).toBeInTheDocument()
        expect(await screen.findByLabelText('GitFlame username')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Link account' })).toBeInTheDocument()

        expect(screen.getByRole('heading', { name: 'Accept pending invite' })).toBeInTheDocument()
        expect(screen.getByLabelText('Repository name')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Accept invite' })).toBeInTheDocument()
    })
})
