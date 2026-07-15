export interface SandboxConfig {
  baseUrl: string
  timeout: number
  token?: string
}

export interface ExecResult {
  stdout: string
  stderr: string
  exit_code: number
  duration_ms: number
}

export interface WriteResult {
  status: string
}

export interface ReadResult {
  content: string
  exists: boolean
}

export class SandboxClient {
  private baseUrl: string
  private timeout: number
  private token: string

  constructor(config: SandboxConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, "")
    this.timeout = config.timeout || 60000
    this.token = config.token || ""
  }

  private authHeaders(extra: Record<string, string> = {}): Record<string, string> {
    return this.token ? { ...extra, Authorization: `Bearer ${this.token}` } : { ...extra }
  }

  async exec(command: string, timeoutSeconds = 60): Promise<ExecResult> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const resp = await fetch(`${this.baseUrl}/exec`, {
        method: "POST",
        headers: this.authHeaders({ "Content-Type": "application/json" }),
        body: JSON.stringify({ command, timeout: timeoutSeconds }),
        signal: controller.signal,
      })

      if (!resp.ok) {
        const text = await resp.text().catch(() => "")
        throw new Error(`Sandbox exec failed: ${resp.status} ${text}`)
      }

      return resp.json() as Promise<ExecResult>
    } finally {
      clearTimeout(timeoutId)
    }
  }

  async writeFile(path: string, content: string): Promise<WriteResult> {
    const resp = await fetch(`${this.baseUrl}/write`, {
      method: "POST",
      headers: this.authHeaders({ "Content-Type": "application/json" }),
      body: JSON.stringify({ path, content }),
    })

    if (!resp.ok) {
      const text = await resp.text().catch(() => "")
      throw new Error(`Sandbox write failed: ${resp.status} ${text}`)
    }

    return resp.json() as Promise<WriteResult>
  }

  async readFile(path: string): Promise<ReadResult> {
    const resp = await fetch(`${this.baseUrl}/read?path=${encodeURIComponent(path)}`, {
      headers: this.authHeaders(),
    })

    if (!resp.ok) {
      const text = await resp.text().catch(() => "")
      throw new Error(`Sandbox read failed: ${resp.status} ${text}`)
    }

    return resp.json() as Promise<ReadResult>
  }

  async health(): Promise<boolean> {
    try {
      const resp = await fetch(`${this.baseUrl}/health`, {
        signal: AbortSignal.timeout(5000),
      })
      return resp.ok
    } catch {
      return false
    }
  }

  async populate(archive: Uint8Array): Promise<{ files: number }> {
    const resp = await fetch(`${this.baseUrl}/populate`, {
      method: "POST",
      headers: this.authHeaders({ "Content-Type": "application/gzip" }),
      body: archive,
    })
    if (!resp.ok) {
      const text = await resp.text().catch(() => "")
      throw new Error(`Sandbox populate failed: ${resp.status} ${text}`)
    }
    return resp.json() as Promise<{ files: number }>
  }
}

// Singleton
let client: SandboxClient | null = null

export function getSandboxClient(): SandboxClient {
  if (!client) {
    const baseUrl = process.env.SANDBOX_URL || "http://sandbox:8080"
    const timeout = parseInt(process.env.SANDBOX_TIMEOUT || "60000")
    const token = process.env.SANDBOX_TOKEN || ""
    client = new SandboxClient({ baseUrl, timeout, token })
  }
  return client
}
