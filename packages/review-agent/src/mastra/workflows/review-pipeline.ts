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

    let files: string[] = []
    try {
      files = await client.listPRFiles(owner, repo, pullNumber)
    } catch (error) {
      console.error("Failed to list PR files:", error)
      // Continue with empty file list
    }

    const diffs: string[] = []
    for (const file of files) {
      try {
        const diff = await client.getFileDiff(owner, repo, pullNumber, file)
        if (diff) {
          diffs.push(`diff --git a/${file} b/${file}\n${diff}`)
        }
      } catch (error) {
        // Skip failed files, continue with others
        console.warn(`Failed to fetch diff for ${file}:`, error)
      }
    }
    const fullDiff = diffs.join("\n")

    let agentsMd = "(absent)"
    try {
      const agentsMdResult = await client.getRawFile(owner, repo, "AGENTS.md", headSha)
      if (agentsMdResult.found) {
        agentsMd = agentsMdResult.content
      }
    } catch (error) {
      console.warn("Failed to fetch AGENTS.md:", error)
    }

    let readmeMd = "(absent)"
    try {
      const readmeMdResult = await client.getRawFile(owner, repo, "README.md", headSha)
      if (readmeMdResult.found) {
        readmeMd = readmeMdResult.content
      }
    } catch (error) {
      console.warn("Failed to fetch README.md:", error)
    }

    return {
      files,
      fullDiff,
      agentsMd,
      readmeMd,
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
    const agent = mastra.getAgent("codeReviewerAgent")

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
