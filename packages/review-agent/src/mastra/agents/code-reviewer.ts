import { Agent } from "@mastra/core/agent"
import { listChangedFiles } from "../../tools/list-changed-files"
import { getPrDiff } from "../../tools/get-pr-diff"
import { readRepositoryFile } from "../../tools/read-repository-file"
import { getPrComments } from "../../tools/get-pr-comments"
import { buildReviewPrompt } from "../../prompt/builder"

// Direct Ollama access bypasses the orchestrator and its arch-router,
// saving ~200-500ms per review and ensuring the coder model is used.
const ollamaUrl = process.env.OLLAMA_BASE_URL
const reviewModel = process.env.REVIEW_MODEL || "qwen2.5-coder-32b"

const modelUrl = ollamaUrl
  ? `${ollamaUrl.replace(/\/$/, "")}/v1`
  : `${process.env.ORCHESTRATOR_URL || "http://orchestrator:8080"}/v1`

export const codeReviewerAgent = new Agent({
  id: "code-reviewer",
  name: "Code Review Agent",
  instructions: buildReviewPrompt(),
  model: {
    id: reviewModel,
    url: modelUrl,
  },
  tools: { listChangedFiles, getPrDiff, readRepositoryFile, getPrComments },
})
