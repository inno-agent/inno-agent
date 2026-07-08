import { Agent } from "@mastra/core/agent"
import { listChangedFiles } from "../../tools/list-changed-files"
import { getPrDiff } from "../../tools/get-pr-diff"
import { readRepositoryFile } from "../../tools/read-repository-file"
import { getPrComments } from "../../tools/get-pr-comments"
import { buildReviewPrompt } from "../../prompt/builder"

export const codeReviewerAgent = new Agent({
  id: "code-reviewer",
  name: "Code Review Agent",
  instructions: buildReviewPrompt(),
  model: {
    id: "custom/review-model",
    url: process.env.ORCHESTRATOR_URL
      ? `${process.env.ORCHESTRATOR_URL}/v1`
      : "http://orchestrator:8080/v1",
  },
  tools: { listChangedFiles, getPrDiff, readRepositoryFile, getPrComments },
})
