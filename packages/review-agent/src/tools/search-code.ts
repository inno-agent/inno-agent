import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"
import { sandboxRunIdFromContext } from "../services/sandbox-run"

export const searchCode = createTool({
  id: "search-code",
  description: "Search for code patterns using ripgrep. Use to find where a function is defined, where a variable is used, or to search for patterns across the codebase.",
  inputSchema: z.object({
    query: z.string().describe("Search pattern (regex supported)"),
    path: z.string().optional().describe("Directory to search in (default: entire workspace)"),
    filePattern: z.string().optional().describe("File pattern to filter (e.g., '*.go', '*.ts')"),
  }),
  outputSchema: z.object({
    results: z.array(z.string()),
    count: z.number(),
  }),
  execute: async ({ query, path, filePattern }, context) => {
    const runId = sandboxRunIdFromContext(context?.requestContext)
    const client = getSandboxClient()

    let cmd = `rg -n "${query.replace(/"/g, '\\"')}"`
    if (filePattern) {
      cmd += ` -g "${filePattern}"`
    }
    if (path) {
      cmd += ` ${path}`
    }

    const result = await client.exec(runId, cmd, 30)
    const lines = result.stdout.split("\n").filter(l => l.trim())

    return {
      results: lines,
      count: lines.length,
    }
  },
})
