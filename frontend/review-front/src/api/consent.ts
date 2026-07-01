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

/**
 * Returns the caller's already-linked GitFlame username, or null if none is
 * linked yet. Used to restore onboarding state after a page reload.
 */
export async function getLinkedGitFlameUsername(): Promise<string | null> {
    try {
        const { data } = await apiClient.get<{ gitflame_username: string }>('/installations/me')
        return data.gitflame_username
    } catch (e) {
        const code = (e as { response?: { status?: number } }).response?.status
        if (code === 404) return null
        throw e
    }
}

/**
 * Confirms the bot's pending collaborator invitation on a repo, so it can be
 * assigned as a PR reviewer without logging into the bot's GitFlame account by hand.
 * The repo owner is resolved server-side from the caller's own linked account.
 */
export async function acceptGitFlameInvite(repoName: string): Promise<void> {
    await apiClient.post('/invitations/accept', {
        repo_name: repoName,
    })
}
