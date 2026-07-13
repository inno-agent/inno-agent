import { Agent } from "@mastra/core/agent"
import { buildReviewPrompt } from "../../prompt/builder"

// Direct Ollama access bypasses the orchestrator and its arch-router,
// saving ~200-500ms per review and ensuring the coder model is used.
const ollamaUrl = process.env.OLLAMA_BASE_URL
const reviewModel = process.env.REVIEW_MODEL || "qwen2.5-coder-32b"

const modelUrl = ollamaUrl
  ? `${ollamaUrl.replace(/\/$/, "")}/v1`
  : `${process.env.ORCHESTRATOR_URL || "http://orchestrator:8080"}/v1`

// Pure text-generation agent — no tools registered.
// Workflow steps (createPlan, investigate, verify) call agent.generate()
// with self-contained prompts. Tools are unused in this flow and would
// cause conflicting instructions if registered.
export const codeReviewerAgent = new Agent({
  id: "code-reviewer",
  name: "Code Review Agent",
  instructions: buildReviewPrompt(),
  model: {
    id: `custom/${reviewModel}`,
    url: modelUrl,
  },
})
