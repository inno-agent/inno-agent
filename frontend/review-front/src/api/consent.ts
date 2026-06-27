import { identityClient } from '@/api/client'

/** Links the logged-in user's GitFlame username to the bot principal record. */
export async function linkGitFlameUsername(gitflameUsername: string): Promise<void> {
    await identityClient.post('/bot/consent', { gitflame_username: gitflameUsername })
}
