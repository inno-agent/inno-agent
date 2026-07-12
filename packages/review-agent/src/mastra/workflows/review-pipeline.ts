import { createWorkflow, createStep } from "@mastra/core/workflows"
import { z } from "zod"
import { getGitFlameClient } from "../../services/gitflame-singleton"

// ─── Schemas ────────────────────────────────────────────────────────────────

const ReviewInputSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  pullNumber: z.number(),
  headSha: z.string(),
})

const ReviewOutputSchema = z.object({
  reviewMarkdown: z.string(),
})

// ─── Step 1: fetchContext (deterministic, no LLM) ──────────────────────────

const MAX_DIFF_SIZE = 100 * 1024 // 100KB per file
const MAX_TOTAL_DIFF = 500 * 1024 // 500KB total

const fetchContextStep = createStep({
  id: "fetch-context",
  inputSchema: ReviewInputSchema,
  outputSchema: z.object({
    files: z.array(z.string()),
    diffs: z.record(z.string()),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  execute: async ({ inputData }) => {
    const { owner, repo, pullNumber, headSha } = inputData
    const client = getGitFlameClient()
    const start = Date.now()

    let files: string[] = []
    try {
      files = await client.listPRFiles(owner, repo, pullNumber)
    } catch (error) {
      console.error("Failed to list PR files:", error)
    }

    const diffs: Record<string, string> = {}
    let totalDiffSize = 0
    const skippedFiles: string[] = []

    for (const file of files) {
      try {
        const diff = await client.getFileDiff(owner, repo, pullNumber, file)
        if (diff) {
          if (diff.length > MAX_DIFF_SIZE) {
            skippedFiles.push(file)
            continue
          }
          if (totalDiffSize + diff.length > MAX_TOTAL_DIFF) {
            skippedFiles.push(file)
            continue
          }
          diffs[file] = diff
          totalDiffSize += diff.length
        }
      } catch (error) {
        console.warn(`Failed to fetch diff for ${file}:`, error)
      }
    }

    if (skippedFiles.length > 0) {
      console.warn(`Skipped ${skippedFiles.length} files due to size limits: ${skippedFiles.join(", ")}`)
    }

    let agentsMd = "(absent)"
    try {
      const result = await client.getRawFile(owner, repo, "AGENTS.md", headSha)
      if (result.found) agentsMd = result.content
    } catch (error) {
      console.warn("Failed to fetch AGENTS.md:", error)
    }

    let readmeMd = "(absent)"
    try {
      const result = await client.getRawFile(owner, repo, "README.md", headSha)
      if (result.found) readmeMd = result.content
    } catch (error) {
      console.warn("Failed to fetch README.md:", error)
    }

    console.log(`fetchContext completed in ${Date.now() - start}ms: ${Object.keys(diffs).length} files, ${totalDiffSize} bytes`)

    return { files, diffs, agentsMd, readmeMd, owner, repo, pullNumber }
  },
})

// ─── Step 2: createPlan (1 LLM call) ──────────────────────────────────────

const PlanItemSchema = z.object({
  file: z.string(),
  priority: z.enum(["critical", "high", "low"]),
  focus: z.string(),
})

const createPlanStep = createStep({
  id: "create-plan",
  inputSchema: z.object({
    files: z.array(z.string()),
    diffs: z.record(z.string()),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  outputSchema: z.object({
    plan: z.array(PlanItemSchema),
    files: z.array(z.string()),
    diffs: z.record(z.string()),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  execute: async ({ inputData, mastra }) => {
    const { files, diffs, agentsMd, readmeMd, owner, repo, pullNumber } = inputData

    // Trivial PR: skip planning, analyze everything
    const totalLines = Object.values(diffs).reduce((sum, d) => {
      return sum + d.split("\n").filter(l => l.startsWith("+") || l.startsWith("-")).length
    }, 0)

    if (totalLines < 20 || files.length <= 3) {
      const plan = files.map(f => ({
        file: f,
        priority: "high" as const,
        focus: "Full review",
      }))
      return { plan, files, diffs, agentsMd, readmeMd, owner, repo, pullNumber }
    }

    const agent = mastra.getAgent("codeReviewerAgent")

    // Build file summary with diff stats
    const fileSummary = files.map(f => {
      const diff = diffs[f] || ""
      const additions = diff.split("\n").filter(l => l.startsWith("+")).length
      const deletions = diff.split("\n").filter(l => l.startsWith("-")).length
      return `- ${f} (+${additions}/-${deletions})`
    }).join("\n")

    const prompt = `You are planning a code review for a pull request.

Changed files (${files.length} files, ~${totalLines} lines changed):
${fileSummary}

Repository context (AGENTS.md):
${agentsMd}

Create an investigation plan. For each file, assign:
- priority: "critical" (security, auth, DB, secrets, input validation), "high" (logic, errors, concurrency), "low" (style, docs, configs, generated code)
- focus: what specifically to look for in this file

Skip files that are purely: lock files, auto-generated, formatting-only changes.
Output ONLY valid JSON: { "plan": [{ "file": "...", "priority": "...", "focus": "..." }] }`

    const start = Date.now()
    const response = await agent.generate(prompt)
    console.log(`createPlan completed in ${Date.now() - start}ms`)

    let plan: z.infer<typeof PlanItemSchema>[]
    try {
      // Extract JSON from response (model may wrap in markdown code blocks)
      const text = response.text.trim()
      const jsonMatch = text.match(/\{[\s\S]*\}/)
      if (!jsonMatch) throw new Error("No JSON found")
      const parsed = JSON.parse(jsonMatch[0])
      plan = Array.isArray(parsed.plan) ? parsed.plan : parsed
    } catch {
      // Fallback: treat all files as high priority
      plan = files.map(f => ({
        file: f,
        priority: "high" as const,
        focus: "Full review",
      }))
    }

    return { plan, files, diffs, agentsMd, readmeMd, owner, repo, pullNumber }
  },
})

// ─── Step 3: investigate (agent loop with chunking) ────────────────────────

function estimateTokens(text: string): number {
  // Rough estimate: ~4 chars per token
  return Math.ceil(text.length / 4)
}

interface PlanItem {
  file: string
  priority: string
  focus: string
  diff?: string
}

function chunkByTokenBudget(plan: PlanItem[], diffs: Record<string, string>, maxTokens: number): PlanItem[][] {
  const chunks: PlanItem[][] = []
  let current: PlanItem[] = []
  let currentTokens = 0

  // Sort by priority: critical first
  const priorityOrder: Record<string, number> = { critical: 0, high: 1, low: 2 }
  const sorted = [...plan].sort((a, b) =>
    (priorityOrder[a.priority] ?? 1) - (priorityOrder[b.priority] ?? 1)
  )

  for (const item of sorted) {
    const diff = diffs[item.file] || ""
    const itemTokens = estimateTokens(diff) + 50 // overhead for instructions

    if (currentTokens + itemTokens > maxTokens && current.length > 0) {
      chunks.push(current)
      current = []
      currentTokens = 0
    }

    current.push({ ...item, diff })
    currentTokens += itemTokens
  }

  if (current.length > 0) {
    chunks.push(current)
  }

  return chunks
}

function buildInvestigatePrompt(
  chunk: PlanItem[],
  owner: string,
  repo: string,
  pullNumber: number,
  agentsMd: string,
  readmeMd: string,
): string {
  const fileList = chunk.map(c => c.file).join(", ")
  const diffsText = chunk.map(c => {
    return `=== ${c.file} (priority: ${c.priority}, focus: ${c.focus}) ===\n${c.diff}`
  }).join("\n\n")

  return `Review PR ${owner}/${repo}#${pullNumber}

Files to analyze: ${fileList}

Context:
=== AGENTS.md ===
${agentsMd}

=== README.md ===
${readmeMd}

Diffs:
${diffsText}

For each file above, analyze the diff focusing on the specified focus area.
Return a JSON array of findings:
[{ "file": "...", "line": 42, "category": "bug|security|performance|suggestion", "severity": "critical|warning|info", "message": "...", "confidence": 0.9 }]

Rules:
- Only report REAL issues, not style preferences
- Be specific: reference exact lines/functions
- If confidence < 0.5, don't report it
- Skip trivial changes (lock files, formatting, generated code)
- If no issues found, return an empty array []

Output ONLY valid JSON.`
}

function parseFindings(text: string): Array<{
  file: string
  line?: number
  category: string
  severity: string
  message: string
  confidence: number
}> {
  try {
    const jsonMatch = text.match(/\[[\s\S]*\]/)
    if (!jsonMatch) return []
    const parsed = JSON.parse(jsonMatch[0])
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

const investigateStep = createStep({
  id: "investigate",
  inputSchema: z.object({
    plan: z.array(PlanItemSchema),
    files: z.array(z.string()),
    diffs: z.record(z.string()),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  outputSchema: z.object({
    findings: z.array(z.object({
      file: z.string(),
      line: z.number().optional(),
      category: z.enum(["bug", "security", "performance", "suggestion"]),
      severity: z.enum(["critical", "warning", "info"]),
      message: z.string(),
      confidence: z.number(),
    })),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  execute: async ({ inputData, mastra }) => {
    const { plan, diffs, agentsMd, readmeMd, owner, repo, pullNumber } = inputData
    const agent = mastra.getAgent("codeReviewerAgent")

    const chunks = chunkByTokenBudget(plan, diffs, 3000)
    const allFindings: z.infer<typeof investigateStep.outputSchema>["findings"] = []

    const start = Date.now()
    for (let i = 0; i < chunks.length; i++) {
      const chunk = chunks[i]
      const prompt = buildInvestigatePrompt(chunk, owner, repo, pullNumber, agentsMd, readmeMd)
      const response = await agent.generate(prompt)
      const findings = parseFindings(response.text)
      allFindings.push(...findings)
      console.log(`investigate chunk ${i + 1}/${chunks.length}: ${findings.length} findings`)
    }
    console.log(`investigate completed in ${Date.now() - start}ms: ${allFindings.length} total findings`)

    return { findings: allFindings, owner, repo, pullNumber }
  },
})

// ─── Step 4: verify (self-critique) ────────────────────────────────────────

const verifyStep = createStep({
  id: "verify",
  inputSchema: z.object({
    findings: z.array(z.object({
      file: z.string(),
      line: z.number().optional(),
      category: z.enum(["bug", "security", "performance", "suggestion"]),
      severity: z.enum(["critical", "warning", "info"]),
      message: z.string(),
      confidence: z.number(),
    })),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  outputSchema: ReviewOutputSchema,
  execute: async ({ inputData, mastra }) => {
    const { findings, owner, repo, pullNumber } = inputData

    if (findings.length === 0) {
      return { reviewMarkdown: "## Review Summary\n\nNo significant issues found in this PR." }
    }

    const agent = mastra.getAgent("codeReviewerAgent")

    const findingsJson = JSON.stringify(findings, null, 2)

    const prompt = `You previously found these issues in a code review for ${owner}/${repo}#${pullNumber}:

${findingsJson}

For each finding, verify:
1. Is this a REAL issue based on the code context, or a false positive?
2. Rate confidence 0.0-1.0
3. Improve the message if needed (be more specific about the fix)

Be strict. Only keep findings you are confident about (confidence >= 0.5).
Output ONLY valid JSON: { "verified": [{ ...finding, confidence: 0.8 }] }`

    const start = Date.now()
    const response = await agent.generate(prompt)
    console.log(`verify completed in ${Date.now() - start}ms`)

    let verified: typeof findings = []
    try {
      const jsonMatch = response.text.match(/\{[\s\S]*\}/)
      if (!jsonMatch) throw new Error("No JSON found")
      const parsed = JSON.parse(jsonMatch[0])
      verified = (Array.isArray(parsed.verified) ? parsed.verified : parsed)
        .filter((f: any) => (f.confidence ?? 0) >= 0.5)
    } catch {
      // If verification fails, keep original findings with original confidence
      verified = findings.filter(f => f.confidence >= 0.5)
    }

    // Render to markdown
    return { reviewMarkdown: renderReview(verified, owner, repo, pullNumber) }
  },
})

// ─── Render helper ──────────────────────────────────────────────────────────

function renderReview(
  findings: Array<{
    file: string
    line?: number
    category: string
    severity: string
    message: string
    confidence: number
  }>,
  owner: string,
  repo: string,
  pullNumber: number,
): string {
  const lines: string[] = []

  lines.push(`## Code Review: ${owner}/${repo}#${pullNumber}`)
  lines.push("")

  if (findings.length === 0) {
    lines.push("No significant issues found. This PR looks good!")
    return lines.join("\n")
  }

  // Group by severity
  const critical = findings.filter(f => f.severity === "critical")
  const warnings = findings.filter(f => f.severity === "warning")
  const info = findings.filter(f => f.severity === "info")

  const renderGroup = (title: string, items: typeof findings) => {
    if (items.length === 0) return
    lines.push(`### ${title}`)
    lines.push("")
    for (const item of items) {
      const lineRef = item.line ? `:${item.line}` : ""
      const conf = Math.round(item.confidence * 100)
      lines.push(`- **\`${item.file}${lineRef}\`** (${item.category}, ${conf}% confidence)`)
      lines.push(`  ${item.message}`)
      lines.push("")
    }
  }

  renderGroup("Critical Issues", critical)
  renderGroup("Warnings", warnings)
  renderGroup("Suggestions", info)

  return lines.join("\n")
}

// ─── Workflow ───────────────────────────────────────────────────────────────

export const reviewPipeline = createWorkflow({
  id: "review-pipeline",
  inputSchema: ReviewInputSchema,
  outputSchema: ReviewOutputSchema,
})
  .then(fetchContextStep)
  .then(createPlanStep)
  .then(investigateStep)
  .then(verifyStep)
  .commit()
