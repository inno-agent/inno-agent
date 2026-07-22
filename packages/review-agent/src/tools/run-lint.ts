import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"
import { sandboxRunIdFromContext } from "../services/sandbox-run"

export const runLint = createTool({
  id: "run-lint",
  description: "Run the project linter. Automatically detects the project type (Go, Node.js) and runs the appropriate linter.",
  inputSchema: z.object({
    language: z.enum(["go", "node", "auto"]).optional().describe("Project language (default: auto-detect)"),
  }),
  outputSchema: z.object({
    success: z.boolean(),
    stdout: z.string(),
    stderr: z.string(),
    durationMs: z.number(),
  }),
  execute: async ({ language }, context) => {
    const runId = sandboxRunIdFromContext(context?.requestContext)
    const client = getSandboxClient()

    let cmd: string
    const lang = language || "auto"

    if (lang === "go") {
      cmd = "golangci-lint run ./... 2>&1 || go vet ./..."
    } else if (lang === "node") {
      cmd = "npx eslint . 2>/dev/null || echo 'No eslint config found'"
    } else {
      cmd = "if [ -f go.mod ]; then golangci-lint run ./... 2>&1 || go vet ./...; elif [ -f package.json ]; then npx eslint . 2>/dev/null; else echo 'No linter detected'; fi"
    }

    const result = await client.exec(runId, cmd, 120)

    return {
      success: result.exit_code === 0,
      stdout: result.stdout,
      stderr: result.stderr,
      durationMs: result.duration_ms,
    }
  },
})
