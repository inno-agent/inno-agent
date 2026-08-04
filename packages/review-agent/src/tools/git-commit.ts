import { createTool } from "@mastra/core/tools"
import { z } from "zod"
import { getSandboxClient } from "../services/sandbox-client"
import { sandboxRunIdFromContext } from "../services/sandbox-run"

// gitCommit lets the agent create incremental commits inside the sandbox as it
// works, instead of one big commit at the end. The sandbox already has a real
// git repo (cloned by cloneAndBranch in the workflow), so git add/commit are
// local operations — they don't touch GitFlame until pushBranch runs.
export const gitCommit = createTool({
  id: "git-commit",
  description:
    "Stage and commit all current changes in the sandbox git repository. " +
    "Call this after completing a distinct logical change to create a separate commit. " +
    "Uses git add -A then git commit. Does nothing (returns success) if there are no changes to commit.",
  inputSchema: z.object({
    message: z.string().describe("Commit message describing what this change does"),
  }),
  outputSchema: z.object({
    success: z.boolean(),
    committed: z.boolean().describe("false if there were no changes to commit"),
    error: z.string().optional().describe("Present only when success is false"),
  }),
  execute: async ({ message }, context) => {
    const runId = sandboxRunIdFromContext(context?.requestContext)
    const client = getSandboxClient()

    // Stage everything the agent has written so far
    const add = await client.exec(runId, "git add -A", 30)
    if (add.exit_code !== 0) {
      return { success: false, committed: false, error: `git add failed: ${add.stderr || add.stdout}` }
    }

    // Check if there's anything staged — avoids empty commits on repeated calls
    const status = await client.exec(runId, "git status --porcelain", 10)
    if (status.exit_code !== 0) {
      return { success: false, committed: false, error: `git status failed: ${status.stderr || status.stdout}` }
    }
    if (status.stdout.trim().length === 0) {
      return { success: true, committed: false }
    }

    // Encode the message before interpolating it into the shell command. A
    // JSON string is not shell-safe because bash still expands $() in double
    // quotes; base64 contains no shell metacharacters.
    const encodedMessage = Buffer.from(message, "utf-8").toString("base64")
    const commit = await client.exec(runId, `printf %s ${encodedMessage} | base64 -d | git commit -q -F -`, 30)
    if (commit.exit_code !== 0) {
      return { success: false, committed: false, error: `git commit failed: ${commit.stderr || commit.stdout}` }
    }

    return { success: true, committed: true }
  },
})
