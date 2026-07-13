import { createWorkflow, createStep } from "@mastra/core/workflows"
import { z } from "zod"
import { getGitFlameClient } from "../../services/gitflame-singleton"

// ─── Future: Deep Analysis Step ─────────────────────────────────────────────
// Tools for deep analysis (listChangedFiles, getPrDiff, readRepositoryFile,
// getPrComments) were removed from the agent to avoid conflicting instructions.
// They can be re-added as a separate "deep-analysis" workflow step that uses
// agent.generate() with tool-calling enabled, for per-file investigation
// with real-time context fetching. See git history for the original implementations.

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

const FindingSchema = z.object({
  file: z.string(),
  line: z.number().optional(),
  category: z.enum(["bug", "security", "performance", "suggestion"]),
  severity: z.enum(["critical", "warning", "info"]),
  message: z.string(),
  confidence: z.number(),
})

type Finding = z.infer<typeof FindingSchema>

const PlanItemSchema = z.object({
  file: z.string(),
  priority: z.enum(["critical", "high", "low"]),
  focus: z.string(),
})

// ─── Constants ──────────────────────────────────────────────────────────────

const MAX_DIFF_SIZE = 100 * 1024 // 100KB per file
const MAX_TOTAL_DIFF = 500 * 1024 // 500KB total
const CHUNK_TOKEN_BUDGET = parseInt(process.env.CHUNK_TOKEN_BUDGET || "12000")
const SYSTEM_PROMPT_OVERHEAD = 500 // review.md + instructions overhead
const CONCURRENT_CHUNKS = parseInt(process.env.CONCURRENT_CHUNKS || "3")

// ─── Helpers ────────────────────────────────────────────────────────────────

function estimateTokens(text: string): number {
  return Math.ceil(text.length / 4)
}

function normalizeFinding(raw: any): Finding {
  const validCategories = ["bug", "security", "performance", "suggestion"]
  const validSeverities = ["critical", "warning", "info"]
  return {
    file: raw.file || "unknown",
    line: raw.line,
    category: validCategories.includes(raw.category) ? raw.category : "suggestion",
    severity: validSeverities.includes(raw.severity) ? raw.severity : "info",
    message: raw.message || "No description",
    confidence: typeof raw.confidence === "number" ? raw.confidence : 0.7,
  }
}

// Fix 3: Non-greedy JSON array extraction
function parseFindings(text: string): Finding[] {
  try {
    let depth = 0
    let start = -1
    for (let i = 0; i < text.length; i++) {
      if (text[i] === "[" && depth === 0) {
        start = i
        depth = 1
      } else if (text[i] === "[") {
        depth++
      } else if (text[i] === "]") {
        depth--
        if (depth === 0 && start >= 0) {
          const candidate = text.slice(start, i + 1)
          try {
            const parsed = JSON.parse(candidate)
            if (Array.isArray(parsed)) {
              return parsed.map(normalizeFinding)
            }
          } catch {
            // not valid JSON, continue scanning
          }
          start = -1
        }
      }
    }
    return []
  } catch {
    return []
  }
}

function parsePlan(text: string): z.infer<typeof PlanItemSchema>[] {
  try {
    const jsonMatch = text.match(/\{[\s\S]*\}/)
    if (!jsonMatch) return []
    const parsed = JSON.parse(jsonMatch[0])
    const plan = Array.isArray(parsed.plan) ? parsed.plan : (Array.isArray(parsed) ? parsed : [])
    return plan.map((item: any) => ({
      file: item.file || "unknown",
      priority: ["critical", "high", "low"].includes(item.priority) ? item.priority : "high",
      focus: item.focus || "Full review",
    }))
  } catch {
    return []
  }
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

  const priorityOrder: Record<string, number> = { critical: 0, high: 1, low: 2 }
  const sorted = [...plan].sort((a, b) =>
    (priorityOrder[a.priority] ?? 1) - (priorityOrder[b.priority] ?? 1)
  )

  for (const item of sorted) {
    const diff = diffs[item.file] || ""
    const itemTokens = estimateTokens(diff) + 50

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
  const diffsText = chunk.map(c => {
    return `=== ${c.file} (priority: ${c.priority}, focus: ${c.focus}) ===\n${c.diff}`
  }).join("\n\n")

  return `Review PR ${owner}/${repo}#${pullNumber}

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

// ─── Step 1: fetchContext (deterministic, no LLM) ──────────────────────────

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

    // Fix 10: Trivial PR — only skip for truly tiny changes
    const totalLines = Object.values(diffs).reduce((sum, d) => {
      return sum + d.split("\n").filter(l => l.startsWith("+") || l.startsWith("-")).length
    }, 0)

    if (totalLines < 10 && files.length <= 1) {
      const plan = files.map(f => ({
        file: f,
        priority: "high" as const,
        focus: "Full review",
      }))
      return { plan, files, diffs, agentsMd, readmeMd, owner, repo, pullNumber }
    }

    const agent = mastra.getAgent("codeReviewerAgent")

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

    let plan = parsePlan(response.text)
    if (plan.length === 0) {
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

// ─── Step 3: investigate (parallel chunks, chunked analysis) ────────────────

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
    findings: z.array(FindingSchema),
    diffs: z.record(z.string()),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  execute: async ({ inputData, mastra }) => {
    const { plan, diffs, agentsMd, readmeMd, owner, repo, pullNumber } = inputData
    const agent = mastra.getAgent("codeReviewerAgent")

    // Fix 5: Increased token budget
    const chunks = chunkByTokenBudget(plan, diffs, CHUNK_TOKEN_BUDGET - SYSTEM_PROMPT_OVERHEAD)
    const allFindings: Finding[] = []

    const start = Date.now()

    // Fix 9: Parallel chunk processing
    for (let i = 0; i < chunks.length; i += CONCURRENT_CHUNKS) {
      const batch = chunks.slice(i, i + CONCURRENT_CHUNKS)
      const batchResults = await Promise.all(
        batch.map(async (chunk) => {
          const prompt = buildInvestigatePrompt(chunk, owner, repo, pullNumber, agentsMd, readmeMd)
          const response = await agent.generate(prompt)
          return parseFindings(response.text)
        })
      )
      for (const findings of batchResults) {
        allFindings.push(...findings)
      }
      console.log(`investigate batch ${Math.floor(i / CONCURRENT_CHUNKS) + 1}/${Math.ceil(chunks.length / CONCURRENT_CHUNKS)}: ${batchResults.reduce((s, f) => s + f.length, 0)} findings`)
    }

    console.log(`investigate completed in ${Date.now() - start}ms: ${allFindings.length} total findings`)

    // Fix 12: Pass diffs through to verify step
    return { findings: allFindings, diffs, agentsMd, readmeMd, owner, repo, pullNumber }
  },
})

// ─── Step 4: verify (self-critique with code context) ──────────────────────

const verifyStep = createStep({
  id: "verify",
  inputSchema: z.object({
    findings: z.array(FindingSchema),
    diffs: z.record(z.string()),
    agentsMd: z.string(),
    readmeMd: z.string(),
    owner: z.string(),
    repo: z.string(),
    pullNumber: z.number(),
  }),
  outputSchema: ReviewOutputSchema,
  execute: async ({ inputData, mastra }) => {
    const { findings, diffs, owner, repo, pullNumber } = inputData

    if (findings.length === 0) {
      return { reviewMarkdown: "## Review Summary\n\nNo significant issues found in this PR." }
    }

    const agent = mastra.getAgent("codeReviewerAgent")

    // Fix 2: Include relevant diffs for verification
    const relevantDiffs = findings.map(f => {
      const diff = diffs[f.file] || ""
      // Take first 50 lines of diff for context
      const truncated = diff.split("\n").slice(0, 50).join("\n")
      return `=== ${f.file} ===\n${truncated}`
    }).join("\n\n")

    const findingsJson = JSON.stringify(findings, null, 2)

    const prompt = `Verify these findings against the actual code:

Findings:
${findingsJson}

Relevant diffs (truncated):
${relevantDiffs}

For each finding:
1. Is this a REAL issue based on the diff, or a false positive?
2. Rate confidence 0.0-1.0
3. Improve the message if needed (be more specific)

Be strict. Only keep findings you are confident about (confidence >= 0.5).
Output ONLY valid JSON: { "verified": [{ "file": "...", "line": 42, "category": "...", "severity": "...", "message": "...", "confidence": 0.8 }] }`

    const start = Date.now()
    const response = await agent.generate(prompt)
    console.log(`verify completed in ${Date.now() - start}ms`)

    let verified: Finding[] = []
    try {
      const jsonMatch = response.text.match(/\{[\s\S]*\}/)
      if (!jsonMatch) throw new Error("No JSON found")
      const parsed = JSON.parse(jsonMatch[0])
      const items = Array.isArray(parsed.verified) ? parsed.verified : (Array.isArray(parsed) ? parsed : [])
      verified = items
        .map(normalizeFinding)
        .filter((f: Finding) => f.confidence >= 0.5)
    } catch {
      // If verification fails, keep original findings with confidence >= 0.5
      verified = findings.filter(f => f.confidence >= 0.5)
    }

    // Render to markdown
    return { reviewMarkdown: renderReview(verified, owner, repo, pullNumber) }
  },
})

// ─── Render helper ──────────────────────────────────────────────────────────

function renderReview(
  findings: Finding[],
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

  const critical = findings.filter(f => f.severity === "critical")
  const warnings = findings.filter(f => f.severity === "warning")
  const info = findings.filter(f => f.severity === "info")

  const renderGroup = (title: string, items: Finding[]) => {
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
