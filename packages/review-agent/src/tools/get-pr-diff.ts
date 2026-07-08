import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getGitFlameClient } from "../services/gitflame-singleton"

export const getPrDiff = createTool({
  id: "get-pr-diff",
  description: "Get the diff of a specific file in a pull request",
  inputSchema: z.object({
    owner: z.string().describe("Repository owner"),
    repo: z.string().describe("Repository name"),
    pullNumber: z.number().describe("Pull request number"),
    filePath: z.string().describe("Path to the file to get diff for"),
  }),
  outputSchema: z.object({
    diff: z.string().describe("Unified diff of the file"),
  }),
  execute: async ({ owner, repo, pullNumber, filePath }) => {
    const client = getGitFlameClient()
    const diff = await client.getFileDiff(owner, repo, pullNumber, filePath)
    return { diff }
  },
})
