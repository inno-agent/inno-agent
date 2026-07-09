import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getToken, login, logout } from '@/auth/auth'
import { apiClient } from '@/api/client'


vi.mock('@/auth/auth', () => ({
    getToken: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
}))

describe('api client (withAuth)', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('adds an Authorization header when a token is present', () => {
        vi.mocked(getToken).mockReturnValue('abc123')

        const onRequest = (apiClient.interceptors.request as any).handlers[0].fulfilled
        const config = onRequest({ headers: {} })

        expect(config.headers.Authorization).toBe('Bearer abc123')
    })

    it('does not add an Authorization header when there is no token', () => {
        vi.mocked(getToken).mockReturnValue(null)

        const onRequest = (apiClient.interceptors.request as any).handlers[0].fulfilled
        const config = onRequest({ headers: {} })

        expect(config.headers.Authorization).toBeUndefined()
    })

    it('logs out and redirects to login on a 401 response error', async () => {
        const onError = (apiClient.interceptors.response as any).handlers[0].rejected
        const error = { response: { status: 401 } }

        await expect(onError(error)).rejects.toBe(error)

        expect(logout).toHaveBeenCalledOnce()
        expect(login).toHaveBeenCalledOnce()
    })

    it('does not log out or redirect on a non-401 response error', async () => {
        const onError = (apiClient.interceptors.response as any).handlers[0].rejected
        const error = { response: { status: 500 } }

        await expect(onError(error)).rejects.toBe(error)

        expect(logout).not.toHaveBeenCalled()
        expect(login).not.toHaveBeenCalled()
    })
})