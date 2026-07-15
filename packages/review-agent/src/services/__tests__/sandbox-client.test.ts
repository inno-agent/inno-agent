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
    await c.exec("echo hi")
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
    await c.exec("echo hi")
    const init = fetchMock.mock.calls[0][1]
    expect(init.headers["Authorization"]).toBeUndefined()
  })
})
