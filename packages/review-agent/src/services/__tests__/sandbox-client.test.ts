import { describe, it, expect, vi, beforeEach } from "vitest"
import { SandboxClient } from "../sandbox-client"

describe("SandboxClient auth", () => {
  beforeEach(() => vi.restoreAllMocks())

  it("sends bearer token on exec", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ stdout: "", stderr: "", exit_code: 0, duration_ms: 1 }),
    })
    vi.stubGlobal("fetch", fetchMock)
    const c = new SandboxClient({ baseUrl: "http://sandbox:8080", timeout: 1000, token: "tok" })
    await c.exec("run-123", "echo hi")
    const init = fetchMock.mock.calls[0][1]
    expect(init.headers["Authorization"]).toBe("Bearer tok")
  })

  it("omits Authorization when no token", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ stdout: "", stderr: "", exit_code: 0, duration_ms: 1 }),
    })
    vi.stubGlobal("fetch", fetchMock)
    const c = new SandboxClient({ baseUrl: "http://sandbox:8080", timeout: 1000, token: "" })
    await c.exec("run-123", "echo hi")
    const init = fetchMock.mock.calls[0][1]
    expect(init.headers["Authorization"]).toBeUndefined()
  })
})

describe("SandboxClient populate", () => {
  beforeEach(() => vi.restoreAllMocks())
  it("POSTs /populate with bearer and gzip body", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ files: 5 }) })
    vi.stubGlobal("fetch", fetchMock)
    const c = new SandboxClient({ baseUrl: "http://sandbox:8080", timeout: 1000, token: "tok" })
    const res = await c.populate("run-123", new Uint8Array([1, 2, 3]))
    expect(fetchMock.mock.calls[0][0]).toContain("http://sandbox:8080/populate?run_id=run-123")
    const init = fetchMock.mock.calls[0][1]
    expect(init.method).toBe("POST")
    expect(init.headers["Authorization"]).toBe("Bearer tok")
    expect(res.files).toBe(5)
  })
})
