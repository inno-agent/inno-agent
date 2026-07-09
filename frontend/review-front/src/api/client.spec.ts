import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { InternalAxiosRequestConfig } from 'axios'
import { getToken, login, logout } from '@/auth/auth'
import { apiClient } from '@/api/client'

vi.mock('@/auth/auth', () => ({
    getToken: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
}))

interface RequestInterceptor {
    fulfilled: (config: InternalAxiosRequestConfig) => InternalAxiosRequestConfig
}

interface ResponseInterceptor {
    rejected: (error: unknown) => Promise<never>
}

function getRequestInterceptor(): RequestInterceptor {
    const manager = apiClient.interceptors.request as unknown as { handlers: RequestInterceptor[] }
    return manager.handlers[0]
}

function getResponseInterceptor(): ResponseInterceptor {
    const manager = apiClient.interceptors.response as unknown as { handlers: ResponseInterceptor[] }
    return manager.handlers[0]
}

function fakeConfig(): InternalAxiosRequestConfig {
    return { headers: {} } as unknown as InternalAxiosRequestConfig
}

describe('api client (withAuth)', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('adds an Authorization header when a token is present', () => {
        vi.mocked(getToken).mockReturnValue('abc123')

        const config = getRequestInterceptor().fulfilled(fakeConfig())

        expect(config.headers.Authorization).toBe('Bearer abc123')
    })

    it('does not add an Authorization header when there is no token', () => {
        vi.mocked(getToken).mockReturnValue(null)

        const config = getRequestInterceptor().fulfilled(fakeConfig())

        expect(config.headers.Authorization).toBeUndefined()
    })

    it('logs out and redirects to login on a 401 response error', async () => {
        const error = { response: { status: 401 } }

        await expect(getResponseInterceptor().rejected(error)).rejects.toBe(error)

        expect(logout).toHaveBeenCalledOnce()
        expect(login).toHaveBeenCalledOnce()
    })

    it('does not log out or redirect on a non-401 response error', async () => {
        const error = { response: { status: 500 } }

        await expect(getResponseInterceptor().rejected(error)).rejects.toBe(error)

        expect(logout).not.toHaveBeenCalled()
        expect(login).not.toHaveBeenCalled()
    })
})
