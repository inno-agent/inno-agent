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
 * Confirms the bot's pending collaborator invitation on a repo, so it can be
 * assigned as a PR reviewer without logging into the bot's GitFlame account by hand.
 * The repo owner is resolved server-side from the caller's own linked account.
 */
export async function acceptGitFlameInvite(repoName: string): Promise<void> {
    await apiClient.post('/invitations/accept', {
        repo_name: repoName,
    })
}
