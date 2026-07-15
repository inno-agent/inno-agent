import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"

export const runBuild = createTool({
  id: "run-build",
  description: "Run the project build. Automatically detects the project type (Go, Node.js) and runs the appropriate build command.",
  inputSchema: z.object({
    language: z.enum(["go", "node", "auto"]).optional().describe("Project language (default: auto-detect)"),
  }),
  outputSchema: z.object({
    success: z.boolean(),
    stdout: z.string(),
    stderr: z.string(),
    durationMs: z.number(),
  }),
  execute: async ({ language }) => {
    const client = getSandboxClient()

    let cmd: string
    const lang = language || "auto"

    if (lang === "go") {
      cmd = "go build ./..."
    } else if (lang === "node") {
      cmd = "npm run build 2>/dev/null || npx tsc --noEmit"
    } else {
      // Auto-detect
      cmd = "if [ -f go.mod ]; then go build ./...; elif [ -f package.json ]; then npm run build 2>/dev/null || npx tsc --noEmit; else echo 'No build system detected'; exit 1; fi"
    }

    const result = await client.exec(cmd, 120)

    return {
      success: result.exit_code === 0,
      stdout: result.stdout,
      stderr: result.stderr,
      durationMs: result.duration_ms,
    }
  },
})
