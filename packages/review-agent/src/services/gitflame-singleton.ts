import { GitFlameClient } from "./gitflame-client.js"

let client: GitFlameClient | null = null

export function getGitFlameClient(): GitFlameClient {
  if (!client) {
    client = new GitFlameClient({
      baseUrl: process.env.GITFLAME_BASE_URL || "",
      token: process.env.GITFLAME_TOKEN || "",
    })
  }
  return client
}
