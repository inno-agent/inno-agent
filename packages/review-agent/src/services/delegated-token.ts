import { RequestContext } from "@mastra/core/di"

// Canonical header the Go consumers use to carry the issue/PR author's
// delegated token, kept separate from Authorization (which carries the shared
// service secret).
export const DELEGATED_TOKEN_HEADER = "X-Delegated-Token"

// Key under which the token lives in the Mastra RequestContext.
export const DELEGATED_TOKEN_KEY = "delegatedToken"

// requestContextFromHeaders builds the RequestContext for a run from the
// incoming request headers. `getHeader` is `c.req.header` in Hono or any
// (name) => string | undefined.
export function requestContextFromHeaders(
  getHeader: (name: string) => string | undefined,
): RequestContext {
  const ctx = new RequestContext()
  const token = getHeader(DELEGATED_TOKEN_HEADER)
  if (token) {
    ctx.set(DELEGATED_TOKEN_KEY, token)
  }
  return ctx
}

// hasDelegatedToken lets a route reject a request that arrived without the
// header before starting a run. The Go consumers always send it on the success
// path (tokensource.Token never returns an empty token without an error), so an
// absent header is a contract violation worth a fast 400 rather than a 500 from
// deep inside a run.
export function hasDelegatedToken(ctx: RequestContext): boolean {
  return ctx.has(DELEGATED_TOKEN_KEY)
}
