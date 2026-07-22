import { describe, it, expect } from "vitest"
import { RequestContext } from "@mastra/core/di"
import { orchestratorModel } from "../model"
import { DELEGATED_TOKEN_KEY } from "../../services/delegated-token"

describe("orchestratorModel", () => {
  it("resolves to the orchestrator url with the token as apiKey", () => {
    const resolve = orchestratorModel("qwen2.5-coder:1.5b")
    const ctx = new RequestContext()
    ctx.set(DELEGATED_TOKEN_KEY, "user-token")

    const cfg = resolve({ requestContext: ctx }) as { id: string; url: string; apiKey: string }

    expect(cfg.apiKey).toBe("user-token")
    expect(cfg.url).toContain("/v1")
    expect(cfg.id).toContain("qwen2.5-coder:1.5b")
  })

  it("throws when the token is absent, rather than calling with none", () => {
    const resolve = orchestratorModel("qwen2.5-coder:1.5b")
    const ctx = new RequestContext()
    expect(() => resolve({ requestContext: ctx })).toThrow(/delegated token/i)
  })
})
