import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"
import { sandboxRunIdFromContext } from "../services/sandbox-run"

export const readSandboxFile = createTool({
  id: "read-sandbox-file",
  description: "Read a file from the sandbox workspace. Use to inspect generated code, check build outputs, or read any file in the project.",
  inputSchema: z.object({
    path: z.string().describe("Relative path to the file in the workspace"),
  }),
  outputSchema: z.object({
    content: z.string(),
    exists: z.boolean(),
  }),
  execute: async ({ path }, context) => {
    const runId = sandboxRunIdFromContext(context?.requestContext)
    const client = getSandboxClient()
    const result = await client.readFile(runId, path)
    return {
      content: result.content,
      exists: result.exists,
    }
  },
})
