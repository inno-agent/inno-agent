import { Mastra } from "@mastra/core"
import { codeReviewerAgent } from "./agents/code-reviewer"
import { codeGeneratorAgent } from "./agents/code-generator"
import { reviewPipeline } from "./workflows/review-pipeline"
import { codegenPipeline } from "./workflows/codegen-pipeline"

export const mastra = new Mastra({
  agents: { codeReviewerAgent, codeGeneratorAgent },
  workflows: { reviewPipeline, codegenPipeline },
})
