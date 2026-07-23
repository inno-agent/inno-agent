import { Agent } from "@mastra/core/agent"
import { buildReviewPrompt } from "../../prompt/builder"
import { readRepositoryFile } from "../../tools/read-repository-file"
import { getPrComments } from "../../tools/get-pr-comments"
import { runCommand } from "../../tools/run-command"
import { runBuild } from "../../tools/run-build"
import { runTests } from "../../tools/run-tests"
import { runLint } from "../../tools/run-lint"
import { searchCode } from "../../tools/search-code"
import { readSandboxFile } from "../../tools/read-sandbox-file"
import { writeSandboxFile } from "../../tools/write-sandbox-file"

// The agent talks to vLLM's OpenAI-compatible API (/v1/chat/completions)
// DIRECTLY. It must NOT go through the orchestrator: the orchestrator exposes
// its own /v1/chat (not /v1/chat/completions), so Mastra's OpenAI client 404s.
// VLLM_BASE_URL selects local vLLM or remote GPU server; default = local container.
// Model: Qwen2.5-Coder-32B for code review (heavy GPU inference).
const vllmUrl = process.env.VLLM_BASE_URL || "http://vllm:8000"
const reviewModel = process.env.REVIEW_MODEL || "qwen2.5-coder-32b"

const modelUrl = `${vllmUrl.replace(/\/$/, "")}/v1`

// Agent with tools for code review, build, and test.
// Tools are available but not forced — model decides when to use them.
export const codeReviewerAgent = new Agent({
  id: "code-reviewer",
  name: "Code Review Agent",
  instructions: buildReviewPrompt(),
  model: {
    id: `custom/${reviewModel}`,
    url: modelUrl,
  },
  tools: {
    // Context tools
    readRepositoryFile,
    getPrComments,
    searchCode,
    // Build/test tools
    runCommand,
    runBuild,
    runTests,
    runLint,
    // File tools
    readSandboxFile,
    writeSandboxFile,
  },
})
