import { RequestContext } from "@mastra/core/di"

// Key under which the per-run sandbox id lives in the RequestContext. Threaded
// the same way P2 threads the delegated token, so a single run's sandbox
// operations all target one isolated /workspace/<run_id> directory.
export const SANDBOX_RUN_KEY = "sandboxRunId"

export function withSandboxRunId(ctx: RequestContext, runId: string): RequestContext {
  ctx.set(SANDBOX_RUN_KEY, runId)
  return ctx
}

// sandboxRunIdFromContext reads the run id, throwing when absent. Like the
// delegated-token resolver, a missing id is a threading bug that must fail loudly
// on the first run rather than silently sharing one workspace across runs.
export function sandboxRunIdFromContext(ctx: RequestContext | undefined): string {
  const id = ctx?.get(SANDBOX_RUN_KEY) as string | undefined
  if (!id) {
    throw new Error(
      "sandbox run id missing from request context; a step or tool failed to thread requestContext",
    )
  }
  return id
}
