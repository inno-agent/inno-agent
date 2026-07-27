export interface ExecFn {
  (command: string): Promise<{ stdout: string; exitCode: number }>
}

export interface ChangedFile {
  path: string
  status: string
}

// EmptyDiffError marks a run that produced no changes. The /codegen handler maps
// it to 422 so the Go consumer classifies it permanent — a retry cannot help a
// run that deterministically changed nothing.
export class EmptyDiffError extends Error {
  constructor(message = "codegen produced no changes") {
    super(message)
    this.name = "EmptyDiffError"
  }
}

// cloneAndBranch shallow-clones the target ref (a real GitFlame remote, so the
// resulting history is a real ancestor of that ref's tip — pushing a new
// branch off it is a clean fast-forward, no --force anywhere in this design)
// and checks out a new local branch for the agent to work on. cloneUrl carries
// embedded credentials (see GitFlameClient.getAuthenticatedCloneUrl) — callers
// must redact it out of any thrown error before it reaches a log or comment.
export async function cloneAndBranch(
  exec: ExecFn,
  opts: { cloneUrl: string; defaultBranch: string; branch: string },
): Promise<void> {
  const clone = await exec(
    `git clone -q --depth 1 --branch ${JSON.stringify(opts.defaultBranch)} ${JSON.stringify(opts.cloneUrl)} .`,
  )
  if (clone.exitCode !== 0) {
    throw new Error(`git clone failed: ${clone.stdout}`)
  }
  const identity = await exec("git config user.email codegen@local && git config user.name codegen")
  if (identity.exitCode !== 0) {
    throw new Error(`git config failed: ${identity.stdout}`)
  }
  const branch = await exec(`git checkout -q -b ${JSON.stringify(opts.branch)}`)
  if (branch.exitCode !== 0) {
    throw new Error(`git checkout -b failed: ${branch.stdout}`)
  }
}

// hasUncommittedChanges is the real-git replacement for the old
// collectAddedAndModified length check: true means the agent changed
// something, false means EmptyDiffError.
export async function hasUncommittedChanges(exec: ExecFn): Promise<boolean> {
  const status = await exec("git status --porcelain")
  if (status.exitCode !== 0) {
    throw new Error(`git status failed: ${status.stdout}`)
  }
  return status.stdout.trim().length > 0
}

// commitAll stages and commits everything the agent changed — a single
// commit, made deterministically by the pipeline rather than left to the
// model's judgment.
export async function commitAll(exec: ExecFn, message: string): Promise<void> {
  const add = await exec("git add -A")
  if (add.exitCode !== 0) {
    throw new Error(`git add failed: ${add.stdout}`)
  }
  const commit = await exec(`git commit -q -m ${JSON.stringify(message)}`)
  if (commit.exitCode !== 0) {
    throw new Error(`git commit failed: ${commit.stdout}`)
  }
}

// listChangedFiles reports what the single commit just made actually touched,
// for the issue comment's "Files changed" list. Metadata only (path + git's
// status letter) — no content, unlike the old Go-side {path, content} contract.
export async function listChangedFiles(exec: ExecFn): Promise<ChangedFile[]> {
  const diff = await exec("git --no-pager diff --name-status HEAD~1..HEAD")
  if (diff.exitCode !== 0) {
    throw new Error(`git diff failed: ${diff.stdout}`)
  }
  const files: ChangedFile[] = []
  for (const line of diff.stdout.split("\n")) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const [status, ...rest] = trimmed.split(/\s+/)
    files.push({ path: rest.join(" "), status })
  }
  return files
}

// pushBranch pushes the current HEAD to <branch> on origin. A rejected push
// (non-fast-forward) means another run's push landed first on this same
// branch — issue reassignment can fire overlapping Process() runs, and
// dedup-by-delivery-ID doesn't stop that. Fetch + rebase onto the new tip and
// retry exactly once; a second failure is a real, permanent conflict.
export async function pushBranch(exec: ExecFn, branch: string): Promise<void> {
  const refspec = JSON.stringify(`HEAD:refs/heads/${branch}`)
  const push = await exec(`git push -q origin ${refspec}`)
  if (push.exitCode === 0) return

  // Explicit refspec: plain `git fetch origin <branch>` only writes FETCH_HEAD
  // when the local branch has no configured upstream (checkout -b never set
  // one) — origin/<branch> below would stay stale/missing without this.
  const fetchRefspec = JSON.stringify(`+${branch}:refs/remotes/origin/${branch}`)
  const fetch = await exec(`git fetch -q origin ${fetchRefspec}`)
  if (fetch.exitCode !== 0) {
    // Nothing to rebase onto — surface the original push failure.
    throw new Error(`git push failed: ${push.stdout}`)
  }
  const rebase = await exec(`git rebase -q origin/${branch}`)
  if (rebase.exitCode !== 0) {
    throw new Error(`git rebase failed: ${rebase.stdout}`)
  }
  const retry = await exec(`git push -q origin ${refspec}`)
  if (retry.exitCode !== 0) {
    throw new Error(`git push retry failed: ${retry.stdout}`)
  }
}
