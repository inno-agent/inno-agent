import { apiClient } from '@/api/client'

/**
 * Links the logged-in user's GitFlame username to their inno-agent account.
 * The bot authenticates independently via service credentials.
 */
export async function linkGitFlameUsername(gitflameUsername: string): Promise<void> {
    await apiClient.post('/installations', {
        gitflame_username: gitflameUsername,
    })
}
