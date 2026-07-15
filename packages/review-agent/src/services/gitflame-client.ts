export interface GitFlameConfig {
  baseUrl: string
  token: string
}

interface PRFile {
  name: string
}

interface FileDiff {
  file_path: string
  patch: string
  is_binary: boolean
}

interface PRComment {
  body: string
  user: { login: string }
}

interface PRDetails {
  body: string
}

export class GitFlameClient {
  private baseUrl: string
  private token: string
  private timeout: number
  private maxRetries: number

  constructor(config: GitFlameConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, "")
    this.token = config.token
    this.timeout = 30000 // 30 seconds
    this.maxRetries = 3
  }

  private async requestWithRetry<T>(path: string, retries = this.maxRetries): Promise<T> {
    for (let attempt = 1; attempt <= retries; attempt++) {
      try {
        return await this.request<T>(path)
      } catch (error: any) {
        const isLastAttempt = attempt === retries
        const isRetryable = error.message?.includes("429") || 
                           error.message?.includes("500") ||
                           error.message?.includes("502") ||
                           error.message?.includes("503")

        if (isLastAttempt || !isRetryable) {
          throw error
        }

        // Exponential backoff: 1s, 2s, 4s
        const delay = Math.min(1000 * Math.pow(2, attempt - 1), 10000)
        console.warn(`GitFlame request failed (attempt ${attempt}/${retries}), retrying in ${delay}ms...`)
        await new Promise(resolve => setTimeout(resolve, delay))
      }
    }
    throw new Error("Max retries exceeded")
  }

  private async request<T>(path: string): Promise<T> {
    const url = `${this.baseUrl}${path}`
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const resp = await fetch(url, {
        headers: { Authorization: `token ${this.token}` },
        signal: controller.signal,
      })

      if (!resp.ok) {
        const text = await resp.text().catch(() => "")
        throw new Error(`GitFlame API error: ${resp.status} ${text}`)
      }

      return resp.json() as Promise<T>
    } finally {
      clearTimeout(timeoutId)
    }
  }

  private async requestRaw(path: string): Promise<string> {
    const url = `${this.baseUrl}${path}`
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const resp = await fetch(url, {
        headers: { Authorization: `token ${this.token}` },
        signal: controller.signal,
      })

      if (resp.status === 404) {
        return ""
      }

      if (!resp.ok) {
        const text = await resp.text().catch(() => "")
        throw new Error(`GitFlame API error: ${resp.status} ${text}`)
      }

      return resp.text()
    } finally {
      clearTimeout(timeoutId)
    }
  }

  async listPRFiles(owner: string, repo: string, pullNumber: number): Promise<string[]> {
    const files = await this.requestWithRetry<PRFile[]>(
      `/api/v1/repos/${owner}/${repo}/pulls/${pullNumber}/files`
    )
    return files.map((f) => f.name)
  }

  async getFileDiff(
    owner: string,
    repo: string,
    pullNumber: number,
    filename: string
  ): Promise<string> {
    const diffs = await this.requestWithRetry<FileDiff[]>(
      `/api/v1/repos/${owner}/${repo}/pulls/${pullNumber}/diff/${encodeURIComponent(filename)}`
    )

    if (!diffs.length || diffs[0].is_binary || !diffs[0].patch) {
      return ""
    }

    return Buffer.from(diffs[0].patch, "base64").toString("utf-8")
  }

  async getRawFile(
    owner: string,
    repo: string,
    path: string,
    ref?: string
  ): Promise<{ content: string; found: boolean }> {
    const query = ref ? `?ref=${encodeURIComponent(ref)}` : ""
    const segments = path.split("/").map(encodeURIComponent).join("/")
    const content = await this.requestRaw(
      `/api/v1/repos/${owner}/${repo}/raw/${segments}${query}`
    )

    if (!content) {
      return { content: "", found: false }
    }

    return { content, found: true }
  }

  async getPRComments(
    owner: string,
    repo: string,
    pullNumber: number
  ): Promise<Array<{ body: string; author: string }>> {
    try {
      const comments = await this.requestWithRetry<PRComment[]>(
        `/api/v1/repos/${owner}/${repo}/issues/${pullNumber}/comments`
      )
      return comments.map((c) => ({
        // Same GitFlame quirk as PR body: an empty comment body comes back as
        // [] (truthy array), not "". Coerce non-strings to "".
        body: typeof c.body === "string" ? c.body : "",
        author: c.user?.login ?? "unknown",
      }))
    } catch {
      return []
    }
  }

  async getPRDescription(
    owner: string,
    repo: string,
    pullNumber: number
  ): Promise<string> {
    try {
      const pr = await this.requestWithRetry<PRDetails>(
        `/api/v1/repos/${owner}/${repo}/pulls/${pullNumber}`
      )
      // GitFlame returns an empty PR body as [] (an array), not "". Since [] is
      // truthy in JS, `pr.body || ""` would leak the array downstream and break
      // the pipeline's string schema — coerce anything non-string to "".
      return typeof pr.body === "string" ? pr.body : ""
    } catch {
      return ""
    }
  }

  // getPRHead returns the head repo + branch of a PR. The archive endpoint
  // resolves branch/tag names but NOT raw commit SHAs (returns 500), so populate
  // must archive by head.ref. head.repo differs from the base repo for fork PRs;
  // fall back to the base repo when head.repo is absent (e.g. deleted fork).
  async getPRHead(
    owner: string,
    repo: string,
    pullNumber: number
  ): Promise<{ headOwner: string; headRepo: string; headRef: string }> {
    const pr = await this.requestWithRetry<{
      head?: { ref?: string; repo?: { name?: string; owner?: { login?: string } } }
    }>(`/api/v1/repos/${owner}/${repo}/pulls/${pullNumber}`)
    const head = pr.head ?? {}
    return {
      headOwner: head.repo?.owner?.login ?? owner,
      headRepo: head.repo?.name ?? repo,
      headRef: typeof head.ref === "string" ? head.ref : "",
    }
  }

  async getRepoArchive(owner: string, repo: string, ref: string): Promise<Uint8Array> {
    const url = `${this.baseUrl}/api/v1/repos/${owner}/${repo}/archive/${encodeURIComponent(ref)}.tar.gz`
    const resp = await fetch(url, {
      headers: { Authorization: `token ${this.token}` },
    })
    if (!resp.ok) {
      const text = await resp.text().catch(() => "")
      throw new Error(`GitFlame archive error: ${resp.status} ${text}`)
    }
    const buf = await resp.arrayBuffer()
    return new Uint8Array(buf)
  }
}
