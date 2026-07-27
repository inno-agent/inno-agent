import { GitFlameClient } from "./gitflame-client.js"

let client: GitFlameClient | null = null

export function getGitFlameClient(): GitFlameClient {
  if (!client) {
    const baseUrl = process.env.GITFLAME_BASE_URL
    if (!baseUrl) throw new Error("GITFLAME_BASE_URL is required")
    client = new GitFlameClient({ baseUrl, token: process.env.GITFLAME_TOKEN || "" })
  }
  return client
}
