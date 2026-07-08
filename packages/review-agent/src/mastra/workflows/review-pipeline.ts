import { createWorkflow, createStep } from "@mastra/core/workflows"
import { z } from "zod"
import { getGitFlameClient } from "../../services/gitflame-singleton"

const ReviewInputSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  pullNumber: z.number(),
  headSha: z.string(),
})

const ReviewOutputSchema = z.object({
  reviewMarkdown: z.string(),
})

const fetchContextStep = createStep({
  id: "fetch-context",
  inputSchema: ReviewInputSchema,
  outputSchema: z.object({
    files: z.array(z.string()),
    fullDiff: z.string(),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  execute: async ({ inputData }) => {
    const { owner, repo, pullNumber, headSha } = inputData
    const client = getGitFlameClient()

    const files = await client.listPRFiles(owner, repo, pullNumber)

    const diffs: string[] = []
    for (const file of files) {
      const diff = await client.getFileDiff(owner, repo, pullNumber, file)
      if (diff) {
        diffs.push(`diff --git a/${file} b/${file}\n${diff}`)
      }
    }
    const fullDiff = diffs.join("\n")

    const agentsMdResult = await client.getRawFile(owner, repo, "AGENTS.md", headSha)
    const readmeMdResult = await client.getRawFile(owner, repo, "README.md", headSha)

    return {
      files,
      fullDiff,
      agentsMd: agentsMdResult.found ? agentsMdResult.content : "(absent)",
      readmeMd: readmeMdResult.found ? readmeMdResult.content : "(absent)",
      owner,
      repo,
      pullNumber,
    }
  },
})

const analyzeStep = createStep({
  id: "analyze",
  inputSchema: z.object({
    files: z.array(z.string()),
    fullDiff: z.string(),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  outputSchema: ReviewOutputSchema,
  execute: async ({ inputData, mastra }) => {
    const agent = mastra.getAgent("code-reviewer")

    const prompt = `Review PR ${inputData.owner}/${inputData.repo}#${inputData.pullNumber}

Changed files: ${inputData.files.join(", ")}

Context files:
=== AGENTS.md ===
${inputData.agentsMd}

=== README.md ===
${inputData.readmeMd}

Diff:
${inputData.fullDiff}`

    const response = await agent.generate(prompt)
    return { reviewMarkdown: response.text }
  },
})

export const reviewPipeline = createWorkflow({
  id: "review-pipeline",
  inputSchema: ReviewInputSchema,
  outputSchema: ReviewOutputSchema,
})
  .then(fetchContextStep)
  .then(analyzeStep)
  .commit()
