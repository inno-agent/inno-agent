import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { RequestContext } from "@mastra/core/di"
import { DELEGATED_TOKEN_KEY } from "../../../services/delegated-token"
import { codeGeneratorAgent } from "../../agents/code-generator"
import { generateFromContext } from "../codegen-pipeline"

// generateFromContext calls generate once, then again for each repair round
// when the output won't parse. Every one of those calls must carry the
// requestContext, or the orchestratorModel resolver throws for lack of a token.
// The repair calls live inside a for loop and are the ones most likely to be
// missed, so we force the loop to run by returning garbage twice.
describe("codegen pipeline repair-path threading", () => {
  let calls: Array<{ hadContext: boolean }>

  beforeEach(() => {
    calls = []
    vi.spyOn(codeGeneratorAgent, "generate").mockImplementation((async (_prompt: any, opts: any) => {
      calls.push({ hadContext: !!opts?.requestContext })
      const text =
        calls.length < 3
          ? "not json at all"
          : `{"summary":"s","files":[{"path":"a.py","content_base64":"cHJpbnQoMSk="}]}`
      return { text } as any
    }) as any)
  })

  afterEach(() => vi.restoreAllMocks())

  it("passes requestContext to the initial call and every repair retry", async () => {
    const ctx = new RequestContext()
    ctx.set(DELEGATED_TOKEN_KEY, "user-token")

    await generateFromContext(
      { owner: "o", repo: "r", issueNumber: 1, defaultBranch: "main", issueType: "issue", title: "t", body: "b", agentsMd: "(absent)", readmeMd: "(absent)" },
      ctx,
    )

    expect(calls.length).toBeGreaterThanOrEqual(3) // initial + 2 repair rounds
    expect(calls.every((c) => c.hadContext)).toBe(true)
  })
})
