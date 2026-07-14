import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"

export const runCommand = createTool({
  id: "run-command",
  description: "Execute a shell command in the sandbox environment. Use for building code, running tests, linting, or any shell operation. Returns stdout, stderr, and exit code.",
  inputSchema: z.object({
    command: z.string().describe("Shell command to execute (e.g., 'go build ./...', 'npm test')"),
    timeout: z.number().optional().describe("Timeout in seconds (default 60, max 300)"),
  }),
  outputSchema: z.object({
    stdout: z.string(),
    stderr: z.string(),
    exitCode: z.number(),
    durationMs: z.number(),
  }),
  execute: async ({ command, timeout }) => {
    const client = getSandboxClient()
    const result = await client.exec(command, timeout || 60)
    return {
      stdout: result.stdout,
      stderr: result.stderr,
      exitCode: result.exit_code,
      durationMs: result.duration_ms,
    }
  },
})
