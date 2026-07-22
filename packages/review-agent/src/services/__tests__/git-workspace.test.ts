import { describe, it, expect } from "vitest"
import { execFileSync } from "node:child_process"
import { mkdtempSync, writeFileSync, rmSync, existsSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import {
  cloneAndBranch,
  hasUncommittedChanges,
  commitAll,
  listChangedFiles,
  pushBranch,
} from "../git-workspace"

function localExec(cwd: string) {
  return async (command: string): Promise<{ stdout: string; exitCode: number }> => {
    try {
      const stdout = execFileSync("bash", ["-c", command], { cwd, encoding: "utf-8" })
      return { stdout, exitCode: 0 }
    } catch (e: any) {
      return { stdout: (e.stdout?.toString() ?? "") + (e.stderr?.toString() ?? ""), exitCode: e.status ?? 1 }
    }
  }
}

// makeBareRemote creates a bare repo (the "GitFlame" side) seeded with one
// commit on `main`, and returns its filesystem path — used as the clone URL
// in place of a real https://...token@... URL. Local git treats a plain path
// exactly like any other remote for clone/fetch/push purposes.
function makeBareRemote(): string {
  const remoteDir = mkdtempSync(join(tmpdir(), "gitws-remote-"))
  execFileSync("git", ["init", "-q", "--bare", "--initial-branch=main", remoteDir])

  const seedDir = mkdtempSync(join(tmpdir(), "gitws-seed-"))
  const run = (cmd: string) => execFileSync("bash", ["-c", cmd], { cwd: seedDir })
  run(`git clone -q ${JSON.stringify(remoteDir)} .`)
  run("git config user.email seed@local && git config user.name seed")
  writeFileSync(join(seedDir, "README.md"), "hello\n")
  run("git add -A && git commit -q -m seed && git push -q origin main")
  rmSync(seedDir, { recursive: true, force: true })
  return remoteDir
}

describe("cloneAndBranch", () => {
  it("clones the default branch and checks out a new branch", async () => {
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

      const files = await listChangedFiles(exec)
      const byPath = Object.fromEntries(files.map((f) => [f.path, f.status]))
      expect(byPath["README.md"]).toBe("M")
      expect(byPath["added.py"]).toBe("A")
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
  it("retries with rebase when push is rejected as non-fast-forward", async () => {
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
