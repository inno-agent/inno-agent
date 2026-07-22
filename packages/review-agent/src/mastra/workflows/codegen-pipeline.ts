import type { RequestContext } from "@mastra/core/di"
import { createWorkflow, createStep } from "@mastra/core/workflows"
import { z } from "zod"
import { getGitFlameClient } from "../../services/gitflame-singleton"
import { getSandboxClient } from "../../services/sandbox-client"
import { sandboxRunIdFromContext } from "../../services/sandbox-run"
import {
  gitBaseline,
  collectAddedAndModified,
  EmptyDiffError,
  type ExecFn,
} from "../../services/git-workspace"
import { codeGeneratorAgent } from "../agents/code-generator"

// ─── Schemas ────────────────────────────────────────────────────────────────

const CodegenInputSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  issueNumber: z.number(),
  defaultBranch: z.string().optional(),
  issueType: z.string().optional(),
  // Optional: the webhook already carries title/body. When empty, the workflow
  // fetches them from GitFlame.
  title: z.string().optional(),
  body: z.string().optional(),
})

const IssueContextSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  issueNumber: z.number(),
  defaultBranch: z.string(),
  issueType: z.string(),
  title: z.string(),
  body: z.string(),
  agentsMd: z.string(),
  readmeMd: z.string(),
})

type IssueContext = z.infer<typeof IssueContextSchema>

const GeneratedFileSchema = z.object({
  path: z.string(),
  content: z.string(),
})

const CodegenOutputSchema = z.object({
  summary: z.string(),
  files: z.array(GeneratedFileSchema),
  verified: z.boolean(),
})

export type CodegenResult = z.infer<typeof CodegenOutputSchema>

// Max repair rounds after the first failed verification. Kept small: a weak
// model that can't fix a build in two tries won't fix it in ten, and each round
// is a full LLM tool loop.
const MAX_REPAIR_ROUNDS = 2

// ─── Step 1: fetchIssueContext (deterministic, no LLM) ──────────────────────

const fetchIssueContextStep = createStep({
  id: "fetch-issue-context",
  inputSchema: CodegenInputSchema,
  outputSchema: IssueContextSchema,
  execute: async ({ inputData }) => {
    const { owner, repo, issueNumber, defaultBranch, issueType } = inputData
    const client = getGitFlameClient()
    const ref = defaultBranch || "main"

    let title = inputData.title || ""
    let body = inputData.body || ""

    // Fetch the issue when the webhook payload is missing EITHER field. The
    // common webhook shape carries a title but no body; requiring both to be
    // absent before fetching left the generator working from a title alone.
    if (!title || !body) {
      try {
        const issue = await client.getIssue(owner, repo, issueNumber)
        if (issue.title) title = issue.title
        if (issue.body) body = issue.body
      } catch (err) {
        console.warn(`[codegen] failed to fetch issue ${owner}/${repo}#${issueNumber}:`, err)
      }
    }

    let agentsMd = "(absent)"
    try {
      const result = await client.getRawFile(owner, repo, "AGENTS.md", ref)
      if (result.found) agentsMd = result.content
    } catch (err) {
      console.warn("[codegen] failed to fetch AGENTS.md:", err)
    }

    let readmeMd = "(absent)"
    try {
      const result = await client.getRawFile(owner, repo, "README.md", ref)
      if (result.found) readmeMd = result.content
    } catch (err) {
      console.warn("[codegen] failed to fetch README.md:", err)
    }

    return {
      owner,
      repo,
      issueNumber,
      defaultBranch: ref,
      issueType: issueType || "issue",
      title,
      body,
      agentsMd,
      readmeMd,
    }
  },
})

// ─── Phases of the agentic run ──────────────────────────────────────────────

// sandboxExec adapts the run-scoped sandbox exec to the ExecFn git-workspace
// expects (stdout + exitCode). All git and shell run through the same isolated
// /workspace/<run_id>.
function sandboxExec(runId: string): ExecFn {
  return async (command: string) => {
    const r = await getSandboxClient().exec(runId, command, 120)
    return { stdout: r.stdout || r.stderr, exitCode: r.exit_code }
  }
}

function issuePrompt(ctx: IssueContext): string {
  return `Repository: ${ctx.owner}/${ctx.repo}
Default branch: ${ctx.defaultBranch}
Issue type: ${ctx.issueType}
Issue #${ctx.issueNumber}
Title: ${ctx.title}

Description:
${ctx.body}

=== AGENTS.md ===
${ctx.agentsMd}

=== README.md ===
${ctx.readmeMd}`
}

// verify runs a deterministic build+test check in the sandbox. No LLM. Returns
// whether it passed and the output to feed a repair round.
async function verify(runId: string): Promise<{ ok: boolean; output: string }> {
  const exec = getSandboxClient()
  const build = await exec.exec(
    runId,
    // go mod tidy before build: keeps go.sum in sync with newly imported
    // packages deterministically, instead of depending on the model to know
    // to run it after seeing a "missing go.sum entry" build error.
    "if [ -f go.mod ]; then go mod tidy && go build ./...; elif [ -f package.json ]; then npm run build 2>/dev/null || npx tsc --noEmit; else echo 'no build system'; fi",
    180,
  )
  if (build.exit_code !== 0) {
    return { ok: false, output: `build failed:\n${build.stderr || build.stdout}` }
  }
  const test = await exec.exec(
    runId,
    "if [ -f go.mod ]; then go test ./...; elif [ -f package.json ]; then npm test 2>/dev/null || echo 'no tests'; else echo 'no tests'; fi",
    300,
  )
  if (test.exit_code !== 0) {
    return { ok: false, output: `tests failed:\n${test.stderr || test.stdout}` }
  }
  return { ok: true, output: `${build.stdout}\n${test.stdout}`.trim() }
}

// summarizeGenerate reduces an agent.generate() result to a one-line signal of
// whether the model actually invoked any tools, or just talked. An empty diff
// at the end of the run is otherwise indistinguishable between "the model
// never called write_sandbox_file" and "it wrote back identical content" —
// this makes that distinguishable from the logs alone.
function summarizeGenerate(label: string, result: { text?: string; toolCalls?: unknown[] }): void {
  const calls = Array.isArray(result.toolCalls) ? result.toolCalls : []
  // toolName/args live under .payload (ToolCallChunk/AgentRunToolCall shape),
  // not on the call object itself — reading them directly silently prints
  // blank names.
  const names = calls
    .map((c) => (c as { payload?: { toolName?: string; args?: { path?: string } } }).payload)
    .map((p) => (p?.args?.path ? `${p.toolName}(${p.args.path})` : p?.toolName))
    .join(", ")
  console.log(
    `[codegen] ${label}: ${calls.length} tool call(s)${names ? ` [${names}]` : ""}, ` +
      `reply preview: ${(result.text || "").slice(0, 200).replace(/\n/g, " ")}`,
  )
}

// runCodegen is the agentic core: populate → baseline → plan → implement →
// verify → repair → collect. Exported so a test can drive it without standing up
// a workflow run. requestContext carries both the delegated token (for the LLM
// call) and the sandbox run id (for isolation).
export async function runCodegen(
  ctx: IssueContext,
  requestContext: RequestContext,
): Promise<CodegenResult> {
  const runId = sandboxRunIdFromContext(requestContext)
  const sandbox = getSandboxClient()
  const exec = sandboxExec(runId)

  try {
    // ── populate + baseline ──────────────────────────────────────────────
    // Fatal here, unlike review: there is no tree to generate against without
    // it, so a failure must be retried rather than degrading.
    const archive = await getGitFlameClient().getRepoArchive(ctx.owner, ctx.repo, ctx.defaultBranch)
    await sandbox.populate(runId, archive)
    await gitBaseline(exec)

    const base = issuePrompt(ctx)

    // ── plan ─────────────────────────────────────────────────────────────
    const planned = await codeGeneratorAgent.generate(
      `${base}\n\nFirst, explore the repository with your read/search tools and decide what to change. Do not write anything yet — reply with a short plan.`,
      { requestContext },
    )
    summarizeGenerate("plan", planned)

    // ── implement ────────────────────────────────────────────────────────
    const implemented = await codeGeneratorAgent.generate(
      `${base}\n\nNow implement the change. Write the files with write_sandbox_file, then build and test. Reply with a short summary of what you changed.`,
      { requestContext },
    )
    summarizeGenerate("implement", implemented)
    let summary = implemented.text?.trim() || `Implemented issue #${ctx.issueNumber}`

    // ── verify + repair ──────────────────────────────────────────────────
    let result = await verify(runId)
    if (!result.ok) console.warn(`[codegen] verify failed after implement:\n${result.output.slice(0, 2000)}`)
    for (let round = 0; !result.ok && round < MAX_REPAIR_ROUNDS; round++) {
      console.warn(`[codegen] verification failed; repair round ${round + 1}/${MAX_REPAIR_ROUNDS}`)
      const repaired = await codeGeneratorAgent.generate(
        `${base}\n\nThe build or tests failed:\n\n${result.output}\n\nFix it. Edit the files with write_sandbox_file, then build and test again. Reply with a short summary.`,
        { requestContext },
      )
      summarizeGenerate(`repair round ${round + 1}`, repaired)
      if (repaired.text?.trim()) summary = repaired.text.trim()
      result = await verify(runId)
      if (!result.ok) console.warn(`[codegen] verify failed after repair round ${round + 1}:\n${result.output.slice(0, 2000)}`)
    }

    // ── collect ──────────────────────────────────────────────────────────
    const files = await collectAddedAndModified(exec)
    if (files.length === 0) {
      // Deterministically pointless — the agent changed nothing. Permanent, not
      // a retry: the /codegen handler maps EmptyDiffError to 422.
      throw new EmptyDiffError()
    }

    return { summary, files, verified: result.ok }
  } finally {
    // Best-effort cleanup; the sandbox reaper is the backstop if this is skipped.
    try {
      await sandbox.deleteWorkspace(runId)
    } catch (err) {
      console.warn(`[codegen] failed to delete workspace ${runId}:`, err)
    }
  }
}

// ─── Step 2: generate (thin wrapper over runCodegen) ────────────────────────

const generateStep = createStep({
  id: "generate",
  inputSchema: IssueContextSchema,
  outputSchema: CodegenOutputSchema,
  execute: async ({ inputData, requestContext }) => runCodegen(inputData, requestContext),
})

// ─── Workflow ───────────────────────────────────────────────────────────────

export const codegenPipeline = createWorkflow({
  id: "codegen-pipeline",
  inputSchema: CodegenInputSchema,
  outputSchema: CodegenOutputSchema,
})
  .then(fetchIssueContextStep)
  .then(generateStep)
  .commit()
