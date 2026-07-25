import { describe, it, expect } from "vitest"
import { RequestContext } from "@mastra/core/di"
import { SANDBOX_RUN_KEY, withSandboxRunId, sandboxRunIdFromContext } from "../sandbox-run"

describe("sandbox run context", () => {
  it("sets and reads the run id", () => {
    const ctx = new RequestContext()
    withSandboxRunId(ctx, "run-123")
    expect(ctx.get(SANDBOX_RUN_KEY)).toBe("run-123")
    expect(sandboxRunIdFromContext(ctx)).toBe("run-123")
  })

  it("throws when the run id is absent, rather than returning empty", () => {
    const ctx = new RequestContext()
    expect(() => sandboxRunIdFromContext(ctx)).toThrow(/sandbox run id/i)
  })

  it("throws when the context itself is missing", () => {
    expect(() => sandboxRunIdFromContext(undefined)).toThrow(/sandbox run id/i)
  })
})
