import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"
import { sandboxRunIdFromContext } from "../services/sandbox-run"

export const writeSandboxFile = createTool({
  id: "write-sandbox-file",
  description: "Write a file to the sandbox workspace. Use to create or modify files during code generation or fixing.",
  inputSchema: z.object({
    path: z.string().describe("Relative path to the file in the workspace"),
    content: z.string().describe("File content to write"),
  }),
  outputSchema: z.object({
    success: z.boolean(),
  }),
  execute: async ({ path, content }, context) => {
    const runId = sandboxRunIdFromContext(context?.requestContext)
    const client = getSandboxClient()
    await client.writeFile(runId, path, content)
    return { success: true }
  },
})
