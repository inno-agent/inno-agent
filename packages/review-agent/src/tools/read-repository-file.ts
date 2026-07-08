import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getGitFlameClient } from "../services/gitflame-singleton"

export const readRepositoryFile = createTool({
  id: "read-repository-file",
  description: "Read a file from the repository at a specific commit",
  inputSchema: z.object({
    owner: z.string().describe("Repository owner"),
    repo: z.string().describe("Repository name"),
    filePath: z.string().describe("Path to the file"),
    ref: z.string().optional().describe("Git ref (commit SHA, branch, tag)"),
  }),
  outputSchema: z.object({
    content: z.string().describe("File content"),
    found: z.boolean().describe("Whether the file was found"),
  }),
  execute: async ({ owner, repo, filePath, ref }) => {
    const client = getGitFlameClient()
    const result = await client.getRawFile(owner, repo, filePath, ref)
    return result
  },
})
