import { execFileSync } from "node:child_process"
import { existsSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { RequestContext } from "@mastra/core/di"
import { afterEach, describe, expect, it, vi } from "vitest"
import * as sandboxClient from "../../services/sandbox-client"
import { SANDBOX_RUN_KEY } from "../../services/sandbox-run"
import { gitCommit } from "../git-commit"

const workspaces: string[] = []

afterEach(() => {
  vi.restoreAllMocks()
  for (const workspace of workspaces.splice(0)) {
    rmSync(workspace, { recursive: true, force: true })
  }
})

function localSandbox(workspace: string) {
  return {
    exec: async (_runId: string, command: string) => {
      try {
        const stdout = execFileSync("bash", ["-c", command], { cwd: workspace, encoding: "utf-8" })
        return { stdout, stderr: "", exit_code: 0, duration_ms: 0 }
      } catch (err: any) {
        return {
          stdout: err.stdout?.toString() ?? "",
          stderr: err.stderr?.toString() ?? "",
          exit_code: err.status ?? 1,
          duration_ms: 0,
        }
      }
    },
  }
}

describe("gitCommit", () => {
  it("keeps shell metacharacters literal in the commit message", async () => {
    const workspace = mkdtempSync(join(tmpdir(), "git-commit-tool-"))
    workspaces.push(workspace)
    execFileSync("git", ["init", "-q"], { cwd: workspace })
    execFileSync("git", ["config", "user.email", "test@example.com"], { cwd: workspace })
    execFileSync("git", ["config", "user.name", "Test"], { cwd: workspace })
    writeFileSync(join(workspace, "README.md"), "change\n")
    vi.spyOn(sandboxClient, "getSandboxClient").mockReturnValue(localSandbox(workspace) as any)

    const requestContext = new RequestContext()
    requestContext.set(SANDBOX_RUN_KEY, "run-123")
    const message = "feat: keep $(touch injected-by-message) literal"
    const result = await gitCommit.execute!({ message }, { requestContext } as any) as { success: boolean; committed: boolean }

    expect(result).toEqual({ success: true, committed: true })
    expect(existsSync(join(workspace, "injected-by-message"))).toBe(false)
    expect(execFileSync("git", ["log", "-1", "--format=%B"], { cwd: workspace, encoding: "utf-8" }).trim()).toBe(message)
  })
})
