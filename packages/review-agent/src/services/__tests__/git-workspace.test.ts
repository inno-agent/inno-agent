import { describe, it, expect } from "vitest"
import { execFileSync } from "node:child_process"
import { mkdtempSync, writeFileSync, rmSync, mkdirSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { collectAddedAndModified, gitBaseline } from "../git-workspace"

// collectAddedAndModified/gitBaseline run git via an injected exec fn so the test
// can point them at a real temp repo. The production caller wires exec to the
// sandbox exec tool. Real git, not a mock — a mocked git tests the mock.
function localExec(cwd: string) {
  return async (command: string): Promise<{ stdout: string; exitCode: number }> => {
    try {
      const stdout = execFileSync("bash", ["-c", command], { cwd, encoding: "utf-8" })
      return { stdout, exitCode: 0 }
    } catch (e: any) {
      return { stdout: e.stdout?.toString() ?? "", exitCode: e.status ?? 1 }
    }
  }
}

describe("collectAddedAndModified", () => {
  it("returns added and modified files, drops deletions and renames", async () => {
    const dir = mkdtempSync(join(tmpdir(), "gitws-"))
    try {
      const run = localExec(dir)
      writeFileSync(join(dir, "keep.txt"), "original\n")
      writeFileSync(join(dir, "gone.txt"), "delete me\n")
      mkdirSync(join(dir, "sub"))
      writeFileSync(join(dir, "sub", "old-name.txt"), "rename me\n")
      await gitBaseline(run)

      // Agent edits: modify keep, add new, delete gone, rename old-name.
      writeFileSync(join(dir, "keep.txt"), "changed\n")
      writeFileSync(join(dir, "added.py"), "print(1)\n")
      await run("rm gone.txt && git mv sub/old-name.txt sub/new-name.txt")

      const files = await collectAddedAndModified(run)
      const byPath = Object.fromEntries(files.map((f) => [f.path, f.content]))

      expect(byPath["keep.txt"]).toBe("changed\n")
      expect(byPath["added.py"]).toBe("print(1)\n")
      // Deletions and renames' old paths are NOT in the output (contract: A/M only).
      expect(byPath["gone.txt"]).toBeUndefined()
      expect(byPath["sub/old-name.txt"]).toBeUndefined()
      // The rename's new path shows up as an add.
      expect(byPath["sub/new-name.txt"]).toBe("rename me\n")
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  })

  it("returns empty when nothing changed", async () => {
    const dir = mkdtempSync(join(tmpdir(), "gitws-"))
    try {
      const run = localExec(dir)
      writeFileSync(join(dir, "a.txt"), "x\n")
      await gitBaseline(run)
      expect(await collectAddedAndModified(run)).toEqual([])
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  })
})
