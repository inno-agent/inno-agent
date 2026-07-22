import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import OnboardingPagе from '@/OnboardingPage'
import { getLinkedGitFlameUsername, eraseGitFlameLink } from '@/api/consent'

afterEach(cleanup)


vi.mock('@/api/consent', () => ({
    getLinkedGitFlameUsername: vi.fn().mockResolvedValue(null),
    linkGitFlameUsername: vi.fn(),
    acceptGitFlameInvite: vi.fn(),
    eraseGitFlameLink: vi.fn(),
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

describe('unlinking', () => {
    it('does not render the unlink button before linking', async () => {
        vi.mocked(getLinkedGitFlameUsername).mockResolvedValue(null)
        render(<OnboardingPagе />)
        await waitFor(() => expect(screen.queryByText(/unlink/i)).not.toBeInTheDocument())
    })

    it('unlinks and returns to the idle form', async () => {
        vi.mocked(getLinkedGitFlameUsername).mockResolvedValue('alice')
        vi.mocked(eraseGitFlameLink).mockResolvedValue(undefined)
        render(<OnboardingPagе />)

        const unlinkButton = await screen.findByRole('button', { name: /unlink/i })
        fireEvent.click(unlinkButton)

        await waitFor(() => expect(eraseGitFlameLink).toHaveBeenCalled())
        // Back to the idle form: the "Link account" button reappears and the
        // username field is empty and editable again.
        expect(await screen.findByRole('button', { name: /link account/i })).toBeInTheDocument()
        expect(screen.getByLabelText(/gitflame username/i)).toHaveValue('')
    })

    it('shows an error and keeps the button visible on failure', async () => {
        vi.mocked(getLinkedGitFlameUsername).mockResolvedValue('alice')
        vi.mocked(eraseGitFlameLink).mockRejectedValue(new Error('network'))
        render(<OnboardingPagе />)

        const unlinkButton = await screen.findByRole('button', { name: /unlink/i })
        fireEvent.click(unlinkButton)

        await waitFor(() => expect(screen.getByText(/something went wrong/i)).toBeInTheDocument())
        // The button must still be there — the user has to be able to retry,
        // not get stuck with no control at all.
        expect(screen.getByRole('button', { name: /unlink/i })).toBeInTheDocument()
    })
})
