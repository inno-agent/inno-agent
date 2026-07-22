import { describe, it, expect } from "vitest"
import {
  DELEGATED_TOKEN_HEADER,
  DELEGATED_TOKEN_KEY,
  requestContextFromHeaders,
  hasDelegatedToken,
} from "../delegated-token"

describe("requestContextFromHeaders", () => {
  it("lifts the delegated token header into the context", () => {
    const ctx = requestContextFromHeaders((name) =>
      name === DELEGATED_TOKEN_HEADER ? "user-token" : undefined,
    )
    expect(ctx.get(DELEGATED_TOKEN_KEY)).toBe("user-token")
  })

  it("leaves the key absent when the header is missing", () => {
    const ctx = requestContextFromHeaders(() => undefined)
    expect(ctx.has(DELEGATED_TOKEN_KEY)).toBe(false)
  })

  it("uses the canonical header name", () => {
    expect(DELEGATED_TOKEN_HEADER).toBe("X-Delegated-Token")
  })

  it("reports token presence for route-level rejection", () => {
    const withToken = requestContextFromHeaders((n) => (n === DELEGATED_TOKEN_HEADER ? "t" : undefined))
    const without = requestContextFromHeaders(() => undefined)
    expect(hasDelegatedToken(withToken)).toBe(true)
    expect(hasDelegatedToken(without)).toBe(false)
  })
})
