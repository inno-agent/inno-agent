import { describe, it, expect, vi, beforeEach } from "vitest"
import { GitFlameClient } from "../gitflame-client"

// The gitflame archive endpoint accepts branch/tag names but not raw commit
// SHAs, so populate archives by PR head.ref (on head.repo, which is the fork
// for cross-repo PRs).
describe("GitFlameClient.getPRHead", () => {
  beforeEach(() => vi.restoreAllMocks())

  function stubJson(payload: unknown) {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => payload,
    }))
  }

  it("returns head repo + ref for a same-repo PR", async () => {
    stubJson({ head: { ref: "testing-ai-pr", repo: { name: "test", owner: { login: "askarr" } } } })
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "t" })
    expect(await c.getPRHead("askarr", "test", 2)).toEqual({
      headOwner: "askarr",
      headRepo: "test",
      headRef: "testing-ai-pr",
    })
  })

  it("returns fork owner/name for a cross-repo PR", async () => {
    stubJson({ head: { ref: "feature", repo: { name: "test", owner: { login: "contributor" } } } })
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "t" })
    expect(await c.getPRHead("askarr", "test", 3)).toEqual({
      headOwner: "contributor",
      headRepo: "test",
      headRef: "feature",
    })
  })

  it("falls back to base repo and empty ref when head.repo is absent", async () => {
    stubJson({ head: {} })
    const c = new GitFlameClient({ baseUrl: "http://gf", token: "t" })
    expect(await c.getPRHead("askarr", "test", 4)).toEqual({
      headOwner: "askarr",
      headRepo: "test",
      headRef: "",
    })
  })
})
