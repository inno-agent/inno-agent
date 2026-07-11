import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { UserManager } from 'oidc-client-ts'
import { createUserManager } from '@/auth/authClient'


vi.mock('oidc-client-ts', () => ({
    UserManager: vi.fn(),
}))

describe('createUserManager (authClient)', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue({
                json: vi.fn().mockResolvedValue({
                    authority: 'https://idp.example.com',
                    client_id: 'client-abc',
                }),
            }),
        )
    })

    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('builds UserManagerSettings from /identity/v1/config', async () => {
        await createUserManager()

        expect(fetch).toHaveBeenCalledWith('/identity/v1/config')
        expect(UserManager).toHaveBeenCalledWith({
            authority: 'https://idp.example.com',
            client_id: 'client-abc',
            redirect_uri: `${window.location.origin}/callback`,
            scope: 'openid email profile',
            response_type: 'code',
        })
    })
})
