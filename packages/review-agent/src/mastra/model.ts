import type { RequestContext } from "@mastra/core/di"
import { DELEGATED_TOKEN_KEY } from "../services/delegated-token"

const orchestratorUrl = process.env.ORCHESTRATOR_URL || "http://orchestrator:8080"
const modelUrl = `${orchestratorUrl.replace(/\/$/, "")}/v1`

// orchestratorModel returns a Mastra model resolver bound to the given model id.
// It reads the per-request delegated token from the RequestContext and puts it
// on the outbound call to the orchestrator as the bearer.
//
// A missing token THROWS on purpose. If a workflow step forgets to thread the
// RequestContext into agent.generate, the call would otherwise proceed with no
// token and silently fall back to service attribution — exactly the quiet
// failure this whole change removes. Failing loudly on the first run is the
// point.
export function orchestratorModel(modelId: string) {
  return ({ requestContext }: { requestContext: RequestContext }) => {
    const token = requestContext.get(DELEGATED_TOKEN_KEY) as string | undefined
    if (!token) {
      throw new Error(
        `orchestratorModel: no delegated token in request context for model ${modelId}; ` +
          `a workflow step likely failed to thread requestContext into agent.generate`,
      )
    }
    return {
      id: `custom/${modelId}` as `${string}/${string}`,
      url: modelUrl,
      apiKey: token,
    }
  }
}
