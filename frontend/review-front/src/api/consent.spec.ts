import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { linkGitFlameUsername, getLinkedGitFlameUsername, acceptGitFlameInvite } from '@/api/consent'
import { apiClient } from '@/api/client'
import { logout, login } from '@/auth/auth'

vi.mock('@/api/client', () => ({
    apiClient: {
        post: vi.fn(),
        get: vi.fn(),
    },
}))

vi.mock('@/auth/auth', () => ({
    logout: vi.fn(),
    login: vi.fn(),
}))

describe('linkGitFlameUsername', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('sends POST /installations with body containing gitflame_username', async () => {
        vi.mocked(apiClient.post).mockResolvedValue({ data: {} })

        await linkGitFlameUsername('testuser')

        expect(apiClient.post).toHaveBeenCalledWith('/installations', {
            gitflame_username: 'testuser',
        })
    })

    it('uses exact field name gitflame_username not username', async () => {
        vi.mocked(apiClient.post).mockResolvedValue({ data: {} })

        await linkGitFlameUsername('myusername')

        const callArgs = vi.mocked(apiClient.post).mock.calls[0]
        expect(callArgs[1]).toHaveProperty('gitflame_username')
        expect(callArgs[1]).not.toHaveProperty('username')
    })

    it('triggers logout and login on 401 error via interceptor', async () => {
        const error = { response: { status: 401 } }
        vi.mocked(apiClient.post).mockRejectedValue(error)

        await expect(linkGitFlameUsername('testuser')).rejects.toEqual(error)
    })
})

describe('getLinkedGitFlameUsername', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('returns gitflame_username from response', async () => {
        const mockResponse = { data: { gitflame_username: 'linkeduser' } }
        vi.mocked(apiClient.get).mockResolvedValue(mockResponse)

        const result = await getLinkedGitFlameUsername()

        expect(result).toBe('linkeduser')
    })

    it('returns null on 404 error', async () => {
        const error = { response: { status: 404 } }
        vi.mocked(apiClient.get).mockRejectedValue(error)

        const result = await getLinkedGitFlameUsername()

        expect(result).toBeNull()
    })

    it('throws error on non-404 errors', async () => {
        const error = { response: { status: 500 } }
        vi.mocked(apiClient.get).mockRejectedValue(error)

        await expect(getLinkedGitFlameUsername()).rejects.toEqual(error)
    })

    it('throws error on 403', async () => {
        const error = { response: { status: 403 } }
        vi.mocked(apiClient.get).mockRejectedValue(error)

        await expect(getLinkedGitFlameUsername()).rejects.toEqual(error)
    })

    it('calls GET /installations/me', async () => {
        const mockResponse = { data: { gitflame_username: 'user' } }
        vi.mocked(apiClient.get).mockResolvedValue(mockResponse)

        await getLinkedGitFlameUsername()

        expect(apiClient.get).toHaveBeenCalledWith('/installations/me')
    })
})

describe('acceptGitFlameInvite', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('sends POST /invitations/accept with body containing repo_name', async () => {
        vi.mocked(apiClient.post).mockResolvedValue({ data: {} })

        await acceptGitFlameInvite('my-repo')

        expect(apiClient.post).toHaveBeenCalledWith('/invitations/accept', {
            repo_name: 'my-repo',
        })
    })

    it('uses exact field name repo_name', async () => {
        vi.mocked(apiClient.post).mockResolvedValue({ data: {} })

        await acceptGitFlameInvite('test-repo')

        const callArgs = vi.mocked(apiClient.post).mock.calls[0]
        expect(callArgs[1]).toHaveProperty('repo_name')
        expect(callArgs[1]).toEqual({ repo_name: 'test-repo' })
    })

    it('throws error on request failure', async () => {
        const error = new Error('Network error')
        vi.mocked(apiClient.post).mockRejectedValue(error)

        await expect(acceptGitFlameInvite('repo')).rejects.toThrow('Network error')
    })
})
