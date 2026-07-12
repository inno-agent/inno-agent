import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getGitFlameClient } from "../services/gitflame-singleton"

// Path allowlist - only these files can be read
const ALLOWED_PATHS = [
  /^AGENTS\.md$/i,
  /^README\.md$/i,
  /^package\.json$/i,
  /^tsconfig\.json$/i,
  /^\.env\.example$/i,
  /^Dockerfile$/i,
  /^docker-compose\.yml$/i,
  /^\.gitignore$/i,
  /^src\/.*\.(ts|tsx|js|jsx)$/,
  /^backend\/.*\.go$/,
  /^docs\/.*\.md$/,
]

function isPathAllowed(filePath: string): boolean {
  return ALLOWED_PATHS.some(pattern => pattern.test(filePath))
}

export const readRepositoryFile = createTool({
  id: "read-repository-file",
  description: "Read a file from the repository at a specific commit. Only allowed paths: AGENTS.md, README.md, package.json, tsconfig.json, source files.",
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
    if (!isPathAllowed(filePath)) {
      return { content: "", found: false }
    }

    const client = getGitFlameClient()
    const result = await client.getRawFile(owner, repo, filePath, ref)
    return result
  },
})
