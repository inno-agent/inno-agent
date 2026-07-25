import { describe, it, expect, vi, beforeEach } from "vitest"
import { GitFlameClient } from "../gitflame-client"

// GitFlame returns an empty PR/comment body as [] (an array), not "". Since []
// is truthy in JS, the old `pr.body || ""` leaked the array downstream and broke
// the review pipeline's string schemas (create-plan: "expected string, received
// array"). getPRDescription/getPRComments must always yield strings.
describe("GitFlameClient string coercion for empty bodies", () => {
  beforeEach(() => vi.restoreAllMocks())

  function stubJson(payload: unknown) {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => payload,
    }))
  }

  it("getPRDescription returns '' when body is an empty array", async () => {
    stubJson({ body: [] })
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "t" })
    const out = await c.getPRDescription("o", "r", 2)
    expect(out).toBe("")
    expect(typeof out).toBe("string")
  })

  it("getPRDescription returns the string body when present", async () => {
    stubJson({ body: "real description" })
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "t" })
    expect(await c.getPRDescription("o", "r", 2)).toBe("real description")
  })

  it("getPRComments coerces array bodies to ''", async () => {
    stubJson([
      { body: [], user: { login: "alice" } },
      { body: "hi", user: { login: "bob" } },
    ])
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "t" })
    const out = await c.getPRComments("o", "r", 2)
    expect(out).toEqual([
      { body: "", author: "alice" },
      { body: "hi", author: "bob" },
    ])
  })
})
