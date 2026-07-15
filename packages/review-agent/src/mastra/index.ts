import { Mastra } from "@mastra/core"
import { codeReviewerAgent } from "./agents/code-reviewer"
import { reviewPipeline } from "./workflows/review-pipeline"

export const mastra = new Mastra({
  agents: { codeReviewerAgent },
  workflows: { reviewPipeline },
})
