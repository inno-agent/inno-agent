import { describe, it, expect, vi, afterEach } from "vitest"
import { RequestContext } from "@mastra/core/di"
import { SANDBOX_RUN_KEY } from "../../../services/sandbox-run"
import * as sandboxClient from "../../../services/sandbox-client"
import * as gitflame from "../../../services/gitflame-singleton"
import * as gitWorkspace from "../../../services/git-workspace"
import { codeGeneratorAgent } from "../../agents/code-generator"
import { runCodegen } from "../codegen-pipeline"

// runCodegen(ctx, requestContext) is the agentic core of the pipeline. The
// workflow step is a thin wrapper, so these tests drive it directly. Two
// contracts matter and are both rot-prone:
//   1. every sandbox call carries the run id (isolation),
//   2. an unfixable verify yields verified:false rather than throwing.

const ISSUE = {
  owner: "o",
  repo: "r",
  issueNumber: 1,
  defaultBranch: "main",
  issueType: "issue",
  title: "t",
  body: "b",
  agentsMd: "(absent)",
  readmeMd: "(absent)",
}

function contextWithRun(runId: string): RequestContext {
  const ctx = new RequestContext()
  ctx.set(SANDBOX_RUN_KEY, runId)
  return ctx
}

// stubEverything wires the collaborators so runCodegen executes end to end
// against fakes. execExit controls what every sandbox exec returns.
function stubEverything(execExit: number) {
  const runIds: string[] = []
  const client = {
    populate: vi.fn(async (runId: string) => {
      runIds.push(runId)
      return { files: 1 }
    }),
    exec: vi.fn(async (runId: string) => {
      runIds.push(runId)
      return { stdout: "out", stderr: "err", exit_code: execExit, duration_ms: 1 }
    }),
    deleteWorkspace: vi.fn(async (runId: string) => {
      runIds.push(runId)
    }),
  }
  vi.spyOn(sandboxClient, "getSandboxClient").mockReturnValue(client as any)
  vi.spyOn(gitflame, "getGitFlameClient").mockReturnValue({
    getRepoArchive: vi.fn(async () => new Uint8Array()),
  } as any)
  vi.spyOn(gitWorkspace, "gitBaseline").mockResolvedValue(undefined)
  vi.spyOn(gitWorkspace, "collectAddedAndModified").mockResolvedValue([
    { path: "a.py", content: "print(1)\n" },
  ])
  vi.spyOn(codeGeneratorAgent, "generate").mockResolvedValue({ text: "did the thing" } as any)
  return { runIds }
}

afterEach(() => vi.restoreAllMocks())

describe("runCodegen", () => {
  it("carries the run id into every sandbox call", async () => {
    const { runIds } = stubEverything(0) // everything passes

    const result = await runCodegen(ISSUE, contextWithRun("run-xyz"))

    expect(runIds.length).toBeGreaterThan(0)
    expect(runIds.every((id) => id === "run-xyz")).toBe(true)
    expect(result.verified).toBe(true)
    expect(result.files).toHaveLength(1)
  })

  it("returns verified:false when verification cannot be fixed", async () => {
    stubEverything(1) // build/test always fail, repair can't help

    const result = await runCodegen(ISSUE, contextWithRun("run-xyz"))

    expect(result.verified).toBe(false)
    // Did not throw — the run still returns its files with an honest flag.
    expect(result.files).toHaveLength(1)
  })

  it("throws EmptyDiffError when the agent changed nothing", async () => {
    stubEverything(0)
    vi.spyOn(gitWorkspace, "collectAddedAndModified").mockResolvedValue([])

    await expect(runCodegen(ISSUE, contextWithRun("run-xyz"))).rejects.toBeInstanceOf(
      gitWorkspace.EmptyDiffError,
    )
  })
})
