import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import App from '@/App'


vi.mock('@/ReviewPage', () => ({
    default: () => <div>review-page-stub</div>
}))

vi.mock('@/OnboardingPage', () => ({
    default: () => <div>onboarding-page-stub</div>
}))

vi.mock('@/auth/auth', () => ({
    getToken: vi.fn().mockReturnValue('fake-token'),
    handleCallback: vi.fn(),
    login: vi.fn(),
}))

describe('App', () => {
    it('renders the app', async () => {
        render(<App />)

        expect(screen.getByRole('button', { name: 'PR Review' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Link account' })).toBeInTheDocument()
        expect(screen.getByText('review-page-stub')).toBeInTheDocument()
    })
})

