import { describe, it, expect } from "vitest"
import { execFileSync, execSync } from "node:child_process"
import { mkdtempSync, writeFileSync, rmSync, existsSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import {
  cloneAndBranch,
  hasUncommittedChanges,
  hasCommitsAhead,
  commitAll,
  listChangedFiles,
  pushBranch,
} from "../git-workspace"

// localExec runs shell commands in the given cwd using the OS-native shell
// (cmd.exe on Windows, /bin/sh on Linux/macOS). The production ExecFn runs in
// a Linux Docker sandbox, but tests run locally — using bash on Windows
// invokes WSL which can't see Windows temp paths, so execSync (native shell)
// is used instead.
function localExec(cwd: string) {
  return async (command: string): Promise<{ stdout: string; exitCode: number }> => {
    try {
      const stdout = execSync(command, { cwd, encoding: "utf-8" })
      return { stdout, exitCode: 0 }
    } catch (e: any) {
      return { stdout: (e.stdout?.toString() ?? "") + (e.stderr?.toString() ?? ""), exitCode: e.status ?? 1 }
    }
  }
}

// makeBareRemote creates a bare repo (the "GitFlame" side) seeded with one
// commit on `main`, and returns its filesystem path — used as the clone URL
// in place of a real https://...token@... URL. Uses direct execFileSync("git")
// calls instead of bash so Windows paths work without SSH misinterpretation.
function makeBareRemote(): string {
  const remoteDir = mkdtempSync(join(tmpdir(), "gitws-remote-"))
  execFileSync("git", ["init", "-q", "--bare", "--initial-branch=main", remoteDir])

  const seedDir = mkdtempSync(join(tmpdir(), "gitws-seed-"))
  execFileSync("git", ["clone", "-q", remoteDir, seedDir])
  execFileSync("git", ["-C", seedDir, "config", "user.email", "seed@local"])
  execFileSync("git", ["-C", seedDir, "config", "user.name", "seed"])
  writeFileSync(join(seedDir, "README.md"), "hello\n")
  execFileSync("git", ["-C", seedDir, "add", "-A"])
  execFileSync("git", ["-C", seedDir, "commit", "-q", "-m", "seed"])
  execFileSync("git", ["-C", seedDir, "push", "-q", "origin", "main"])
  rmSync(seedDir, { recursive: true, force: true })
  return remoteDir
}

describe("cloneAndBranch", () => {
  it("clones the default branch and checks out a new branch", { timeout: 30000 }, async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "main", branch: "innoagent-issue-1" })

      const branch = await exec("git rev-parse --abbrev-ref HEAD")
      expect(branch.stdout.trim()).toBe("innoagent-issue-1")
      expect(existsSync(join(workDir, "README.md"))).toBe(true)
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })

  it("throws when the ref does not exist", async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await expect(
        cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "no-such-branch", branch: "b" }),
      ).rejects.toThrow(/git clone failed/)
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })
})

describe("hasUncommittedChanges / commitAll / listChangedFiles", () => {
  it("reports changes, commits them, and lists path+status", async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "main", branch: "innoagent-issue-1" })

      expect(await hasUncommittedChanges(exec)).toBe(false)

      writeFileSync(join(workDir, "README.md"), "changed\n")
      writeFileSync(join(workDir, "added.py"), "print(1)\n")
      expect(await hasUncommittedChanges(exec)).toBe(true)

      await commitAll(exec, "feat: test commit")

      expect(await hasUncommittedChanges(exec)).toBe(false)

      const files = await listChangedFiles(exec, "main")
      const byPath = Object.fromEntries(files.map((f) => [f.path, f.status]))
      expect(byPath["README.md"]).toBe("M")
      expect(byPath["added.py"]).toBe("A")
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })
})

describe("hasCommitsAhead", () => {
  it("returns false right after clone (no commits ahead)", async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "main", branch: "innoagent-issue-1" })

      expect(await hasCommitsAhead(exec, "main")).toBe(false)
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })

  it("returns true after making a commit", async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "main", branch: "innoagent-issue-1" })

      writeFileSync(join(workDir, "new.txt"), "content\n")
      await commitAll(exec, "feat: add new file")

      expect(await hasCommitsAhead(exec, "main")).toBe(true)
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })

  it("listChangedFiles covers multiple commits against base", async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "main", branch: "innoagent-issue-1" })

      // First commit
      writeFileSync(join(workDir, "a.txt"), "1\n")
      await commitAll(exec, "feat: add a")
      // Second commit
      writeFileSync(join(workDir, "b.txt"), "2\n")
      await commitAll(exec, "feat: add b")

      const files = await listChangedFiles(exec, "main")
      const byPath = Object.fromEntries(files.map((f) => [f.path, f.status]))
      expect(byPath["a.txt"]).toBe("A")
      expect(byPath["b.txt"]).toBe("A")
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })
})

describe("pushBranch", () => {
  it("pushes a brand new branch", async () => {
    const remote = makeBareRemote()
    const workDir = mkdtempSync(join(tmpdir(), "gitws-work-"))
    try {
      const exec = localExec(workDir)
      await cloneAndBranch(exec, { cloneUrl: remote, defaultBranch: "main", branch: "innoagent-issue-1" })
      writeFileSync(join(workDir, "a.txt"), "1\n")
      await commitAll(exec, "add a")

      await pushBranch(exec, "innoagent-issue-1")

      const checkDir = mkdtempSync(join(tmpdir(), "gitws-check-"))
      execFileSync("git", ["clone", "-q", "--branch", "innoagent-issue-1", remote, checkDir])
      expect(existsSync(join(checkDir, "a.txt"))).toBe(true)
      rmSync(checkDir, { recursive: true, force: true })
    } finally {
      rmSync(workDir, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })

  // The real race this session hit twice in production: issue reassignment
  // fires overlapping Process() runs, both clone the same branch tip, both
  // try to push. The loser must rebase onto the winner and land its own
  // commit on top, not lose its work.
  it("retries with rebase when push is rejected as non-fast-forward", { timeout: 30000 }, async () => {
    const remote = makeBareRemote()
    const dirA = mkdtempSync(join(tmpdir(), "gitws-a-"))
    const dirB = mkdtempSync(join(tmpdir(), "gitws-b-"))
    try {
      const execA = localExec(dirA)
      const execB = localExec(dirB)
      const branch = "innoagent-issue-1"

      await cloneAndBranch(execA, { cloneUrl: remote, defaultBranch: "main", branch })
      await cloneAndBranch(execB, { cloneUrl: remote, defaultBranch: "main", branch })

      writeFileSync(join(dirA, "a.txt"), "from A\n")
      await commitAll(execA, "from A")
      await pushBranch(execA, branch)

      writeFileSync(join(dirB, "b.txt"), "from B\n")
      await commitAll(execB, "from B")
      await pushBranch(execB, branch) // A already landed — must rebase + retry, not throw

      const checkDir = mkdtempSync(join(tmpdir(), "gitws-check-"))
      execFileSync("git", ["clone", "-q", "--branch", branch, remote, checkDir])
      expect(existsSync(join(checkDir, "a.txt"))).toBe(true)
      expect(existsSync(join(checkDir, "b.txt"))).toBe(true)
      rmSync(checkDir, { recursive: true, force: true })
    } finally {
      rmSync(dirA, { recursive: true, force: true })
      rmSync(dirB, { recursive: true, force: true })
      rmSync(remote, { recursive: true, force: true })
    }
  })
})
