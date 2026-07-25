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
import { orchestratorModel } from "../model"

// The agent talks to the orchestrator's OpenAI-compatible /v1/chat/completions
// endpoint, carrying the PR author's delegated token as the bearer. The token
// is read from the RequestContext; if absent, the model resolver throws rather
// than falling back to service attribution (see orchestratorModel in model.ts).
// Model selection is controlled via REVIEW_MODEL; default is 1.5b locally.
const reviewModel = process.env.REVIEW_MODEL || "qwen2.5-coder:1.5b"

// Agent with tools for code review, build, and test.
// Tools are available but not forced — model decides when to use them.
export const codeReviewerAgent = new Agent({
  id: "code-reviewer",
  name: "Code Review Agent",
  instructions: buildReviewPrompt(),
  model: orchestratorModel(reviewModel),
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
