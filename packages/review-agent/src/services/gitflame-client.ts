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

export class GitFlameClient {
  private baseUrl: string
  private token: string

  constructor(config: GitFlameConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, "")
    this.token = config.token
  }

  private async request<T>(path: string): Promise<T> {
    const url = `${this.baseUrl}${path}`
    const resp = await fetch(url, {
      headers: { Authorization: `token ${this.token}` },
    })

    if (!resp.ok) {
      const text = await resp.text().catch(() => "")
      throw new Error(`GitFlame API error: ${resp.status} ${text}`)
    }

    return resp.json() as Promise<T>
  }

  private async requestRaw(path: string): Promise<string> {
    const url = `${this.baseUrl}${path}`
    const resp = await fetch(url, {
      headers: { Authorization: `token ${this.token}` },
    })

    if (resp.status === 404) {
      return ""
    }

    if (!resp.ok) {
      const text = await resp.text().catch(() => "")
      throw new Error(`GitFlame API error: ${resp.status} ${text}`)
    }

    return resp.text()
  }

  async listPRFiles(owner: string, repo: string, pullNumber: number): Promise<string[]> {
    const files = await this.request<PRFile[]>(
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
    const diffs = await this.request<FileDiff[]>(
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
      const comments = await this.request<PRComment[]>(
        `/api/v1/repos/${owner}/${repo}/issues/${pullNumber}/comments`
      )
      return comments.map((c) => ({
        body: c.body,
        author: c.user?.login ?? "unknown",
      }))
    } catch {
      return []
    }
  }
}
