import { apiClient } from '@/api/client'
import { getRefreshToken } from '@/auth/auth'

/**
 * Links the logged-in user's GitFlame username to the review installation,
 * handing review-api the user's refresh token so the bot can act on their
 * behalf via identity's generic /refresh flow.
 */
export async function linkGitFlameUsername(gitflameUsername: string): Promise<void> {
    const refreshToken = getRefreshToken()
    if (!refreshToken) {
        throw new Error('missing_refresh_token')
    }

    await apiClient.post('/installations', {
        gitflame_username: gitflameUsername,
        refresh_token: refreshToken,
    })
}
