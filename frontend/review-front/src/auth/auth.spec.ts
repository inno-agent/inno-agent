import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createUserManager } from '@/auth/authClient'
import { getToken, getRefreshToken, logout, handleCallback } from '@/auth/auth'


vi.mock('@/auth/authClient', () => ({
    createUserManager: vi.fn(),
}))

const TOKEN_KEY = 'aicore_token'
const USER_ID_KEY = 'aicore_user_id'
const REFRESH_KEY = 'aicore_refresh_token'

describe('auth', () => {
    beforeEach(() => {
        localStorage.clear()
        sessionStorage.clear()
        vi.clearAllMocks()
    })

    describe('getToken', () => {
        it('returns the token stored in localStorage', () => {
            localStorage.setItem(TOKEN_KEY, 'stored-token')

            expect(getToken()).toBe('stored-token')
        })

        it('returns null when no token is stored', () => {
            expect(getToken()).toBeNull()
        })
    })

    describe('getRefreshToken', () => {
        it('returns the refresh token stored in sessionStorage', () => {
            sessionStorage.setItem(REFRESH_KEY, 'stored-refresh')

            expect(getRefreshToken()).toBe('stored-refresh')
        })

        it('returns null when no refresh token is stored', () => {
            expect(getRefreshToken()).toBeNull()
        })
    })

    describe('logout', () => {
        it('clears the token, user id, and refresh token', () => {
            localStorage.setItem(TOKEN_KEY, 't')
            localStorage.setItem(USER_ID_KEY, 'u')
            sessionStorage.setItem(REFRESH_KEY, 'r')

            logout()

            expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
            expect(localStorage.getItem(USER_ID_KEY)).toBeNull()
            expect(sessionStorage.getItem(REFRESH_KEY)).toBeNull()
        })
    })

    describe('handleCallback', () => {
        const replaceMock = vi.fn()

        beforeEach(() => {
            vi.stubGlobal('fetch', vi.fn())
            vi.stubGlobal('atob', vi.fn().mockReturnValue(JSON.stringify({ sub: 'user-1' })))
            Object.defineProperty(window, 'location', {
                configurable: true,
                value: { ...window.location, replace: replaceMock },
            })
        })

        afterEach(() => {
            vi.unstubAllGlobals()
        })

        it('stores the access token, user id, and refresh token on success', async () => {
            vi.mocked(createUserManager).mockResolvedValue({
                signinRedirectCallback: vi.fn().mockResolvedValue({ id_token: 'header.payload.sig' }),
            } as any)
            vi.mocked(fetch).mockResolvedValue({
                ok: true,
                json: vi.fn().mockResolvedValue({ access_token: 'a.b.c', refresh_token: 'refresh-1' }),
            } as any)

            await handleCallback()

            expect(localStorage.getItem(TOKEN_KEY)).toBe('a.b.c')
            expect(localStorage.getItem(USER_ID_KEY)).toBe('user-1')
            expect(sessionStorage.getItem(REFRESH_KEY)).toBe('refresh-1')
            expect(replaceMock).toHaveBeenCalledWith('/')
        })

        it('still redirects to / when the token exchange fails', async () => {
            vi.mocked(createUserManager).mockResolvedValue({
                signinRedirectCallback: vi.fn().mockResolvedValue({ id_token: 'header.payload.sig' }),
            } as any)
            vi.mocked(fetch).mockResolvedValue({
                ok: false,
                status: 500,
                json: vi.fn(),
            } as any)

            await handleCallback()

            expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
            expect(replaceMock).toHaveBeenCalledWith('/')
        })
    })
})
