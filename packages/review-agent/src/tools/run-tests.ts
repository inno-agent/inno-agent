import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"
import { sandboxRunIdFromContext } from "../services/sandbox-run"

export const runTests = createTool({
  id: "run-tests",
  description: "Run the project tests. Automatically detects the project type (Go, Node.js) and runs the appropriate test command.",
  inputSchema: z.object({
    language: z.enum(["go", "node", "auto"]).optional().describe("Project language (default: auto-detect)"),
    pattern: z.string().optional().describe("Test pattern/filter (e.g., 'TestReview', 'src/**/*.test.ts')"),
  }),
  outputSchema: z.object({
    success: z.boolean(),
    stdout: z.string(),
    stderr: z.string(),
    durationMs: z.number(),
  }),
  execute: async ({ language, pattern }, context) => {
    const runId = sandboxRunIdFromContext(context?.requestContext)
    const client = getSandboxClient()

    let cmd: string
    const lang = language || "auto"

    if (lang === "go") {
      cmd = pattern ? `go test -run ${pattern} ./...` : "go test ./..."
    } else if (lang === "node") {
      cmd = pattern ? `npx vitest run ${pattern}` : "npx vitest run"
    } else {
      cmd = "if [ -f go.mod ]; then go test ./...; elif [ -f package.json ]; then npx vitest run; else echo 'No test framework detected'; exit 1; fi"
    }

    const result = await client.exec(runId, cmd, 180)

    return {
      success: result.exit_code === 0,
      stdout: result.stdout,
      stderr: result.stderr,
      durationMs: result.duration_ms,
    }
  },
})
