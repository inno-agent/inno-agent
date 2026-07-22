export interface ExecFn {
  (command: string): Promise<{ stdout: string; exitCode: number }>
}

export interface CollectedFile {
  path: string
  content: string
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

// gitBaseline commits the populated tree so a later diff reports exactly what the
// agent changed. Runs after populate, before the agent touches anything. The
// identity is local and throwaway; --allow-empty so an empty archive still gives
// a HEAD to diff against.
export async function gitBaseline(exec: ExecFn): Promise<void> {
  const res = await exec(
    "git init -q && git config user.email codegen@local && git config user.name codegen && git add -A && git commit -q -m baseline --allow-empty",
  )
  if (res.exitCode !== 0) {
    throw new Error(`git baseline failed: ${res.stdout}`)
  }
}

// collectAddedAndModified returns the added and modified files after the agent's
// edits, reading each file's current content. Deletions (D) and renames' old
// paths are dropped — the Go contract carries {path, content} and cannot express
// a removal. --no-renames turns a rename into D(old)+A(new), so the new file is
// collected as an add and the old path never appears. `add -A -N` stages new
// files as intent-to-add so they surface as A in --name-status.
export async function collectAddedAndModified(exec: ExecFn): Promise<CollectedFile[]> {
  const status = await exec(
    "git -c core.quotepath=false add -A -N . && git --no-pager diff --name-status --no-renames HEAD",
  )
  if (status.exitCode !== 0) {
    throw new Error(`git diff failed: ${status.stdout}`)
  }

  const files: CollectedFile[] = []
  for (const line of status.stdout.split("\n")) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const [code, ...rest] = trimmed.split(/\s+/)
    const path = rest.join(" ")
    if (code === "A" || code === "M") {
      const read = await exec(`cat -- ${JSON.stringify(path)}`)
      if (read.exitCode === 0) {
        files.push({ path, content: read.stdout })
      }
    } else {
      console.warn(`[codegen] dropping non-add/modify change ${code} ${path}`)
    }
  }
  return files
}
