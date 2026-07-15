import { describe, it, expect, vi, beforeEach } from "vitest"
import { GitFlameClient } from "../gitflame-client"

describe("GitFlameClient.getRepoArchive", () => {
  beforeEach(() => vi.restoreAllMocks())
  it("GETs the archive endpoint with token and returns bytes", async () => {
    const bytes = new Uint8Array([1, 2, 3])
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      arrayBuffer: async () => bytes.buffer,
    })
    vi.stubGlobal("fetch", fetchMock)
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "tok" })
    const out = await c.getRepoArchive("o", "r", "abc123")
    expect(fetchMock.mock.calls[0][0]).toBe("http://gf/api/v1/repos/o/r/archive/abc123.tar.gz")
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe("token tok")
    expect(Array.from(out)).toEqual([1, 2, 3])
  })
})
