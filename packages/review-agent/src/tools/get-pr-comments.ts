import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getGitFlameClient } from "../services/gitflame-singleton"

export const getPrComments = createTool({
  id: "get-pr-comments",
  description: "Get existing comments on a pull request",
  inputSchema: z.object({
    owner: z.string().describe("Repository owner"),
    repo: z.string().describe("Repository name"),
    pullNumber: z.number().describe("Pull request number"),
  }),
  outputSchema: z.object({
    comments: z.array(
      z.object({
        body: z.string(),
        author: z.string(),
      })
    ),
  }),
  execute: async ({ owner, repo, pullNumber }) => {
    const client = getGitFlameClient()
    const comments = await client.getPRComments(owner, repo, pullNumber)
    return { comments }
  },
})
