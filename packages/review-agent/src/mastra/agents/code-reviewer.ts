import { Agent } from "@mastra/core/agent"
import { buildReviewPrompt } from "../../prompt/builder"
import { readRepositoryFile } from "../../tools/read-repository-file"
import { getPrComments } from "../../tools/get-pr-comments"

// Direct Ollama access bypasses the orchestrator and its arch-router,
// saving ~200-500ms per review and ensuring the coder model is used.
const ollamaUrl = process.env.OLLAMA_BASE_URL
const reviewModel = process.env.REVIEW_MODEL || "qwen2.5-coder-32b"

const modelUrl = ollamaUrl
  ? `${ollamaUrl.replace(/\/$/, "")}/v1`
  : `${process.env.ORCHESTRATOR_URL || "http://orchestrator:8080"}/v1`

// Agent with optional tools for deep analysis.
// Tools are available but not forced — model decides when to use them.
// readRepositoryFile: read types, imports, related code for context
// getPrComments: check existing discussion to avoid duplicate comments
export const codeReviewerAgent = new Agent({
  id: "code-reviewer",
  name: "Code Review Agent",
  instructions: buildReviewPrompt(),
  model: {
    id: `custom/${reviewModel}`,
    url: modelUrl,
  },
  tools: { readRepositoryFile, getPrComments },
})
