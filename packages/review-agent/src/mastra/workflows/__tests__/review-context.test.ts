import { describe, it, expect } from "vitest"
import { RequestContext } from "@mastra/core/di"
import { orchestratorModel } from "../../model"
import { DELEGATED_TOKEN_KEY } from "../../../services/delegated-token"

// The review pipeline threads the delegated token into every agent.generate.
// The orchestratorModel resolver throws when the token is absent, so a step
// that forgot to pass requestContext fails loudly at run time. This test locks
// the resolver contract the pipeline depends on; the threading itself is
// verified by tsc plus the audit that all three generate calls pass
// { requestContext } — Mastra's types do not make an omitted context a compile
// error, so there is no cheaper structural check.
describe("review pipeline model threading contract", () => {
  it("resolves with a token and throws without one", () => {
    const resolve = orchestratorModel("m")

    const withToken = new RequestContext()
    withToken.set(DELEGATED_TOKEN_KEY, "t")
    expect(() => resolve({ requestContext: withToken })).not.toThrow()

    const without = new RequestContext()
    expect(() => resolve({ requestContext: without })).toThrow()
  })
})
