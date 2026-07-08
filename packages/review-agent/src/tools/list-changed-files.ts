import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getGitFlameClient } from "../services/gitflame-singleton"

export const listChangedFiles = createTool({
  id: "list-changed-files",
  description: "List files changed in a pull request",
  inputSchema: z.object({
    owner: z.string().describe("Repository owner"),
    repo: z.string().describe("Repository name"),
    pullNumber: z.number().describe("Pull request number"),
  }),
  outputSchema: z.object({
    files: z.array(z.string()).describe("List of changed file paths"),
  }),
  execute: async ({ owner, repo, pullNumber }) => {
    const client = getGitFlameClient()
    const files = await client.listPRFiles(owner, repo, pullNumber)
    return { files }
  },
})
