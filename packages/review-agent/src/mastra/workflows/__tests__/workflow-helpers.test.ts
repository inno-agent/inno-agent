import { describe, it, expect } from "vitest"

// Test the helper functions from review-pipeline
// We extract them for testing since they're pure functions

function estimateTokens(text: string): number {
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

describe("estimateTokens", () => {
  it("should estimate ~4 chars per token", () => {
    expect(estimateTokens("hello")).toBe(2) // 5 chars / 4 = 2
    expect(estimateTokens("a".repeat(100))).toBe(25)
  })

  it("should handle empty string", () => {
    expect(estimateTokens("")).toBe(0)
  })
})

describe("chunkByTokenBudget", () => {
  it("should group files by token budget", () => {
    const plan: PlanItem[] = [
      { file: "a.ts", priority: "high", focus: "logic" },
      { file: "b.ts", priority: "high", focus: "errors" },
      { file: "c.ts", priority: "low", focus: "style" },
    ]
    const diffs: Record<string, string> = {
      "a.ts": "a".repeat(400), // ~100 tokens + 50 overhead = 150
      "b.ts": "b".repeat(400), // ~150
      "c.ts": "c".repeat(400), // ~150
    }

    const chunks = chunkByTokenBudget(plan, diffs, 300)
    expect(chunks.length).toBe(2) // 150 + 150 = 300, then new chunk for 150
  })

  it("should sort by priority (critical first)", () => {
    const plan: PlanItem[] = [
      { file: "low.ts", priority: "low", focus: "style" },
      { file: "critical.ts", priority: "critical", focus: "security" },
      { file: "high.ts", priority: "high", focus: "logic" },
    ]
    const diffs: Record<string, string> = {
      "low.ts": "x".repeat(100),
      "critical.ts": "x".repeat(100),
      "high.ts": "x".repeat(100),
    }

    const chunks = chunkByTokenBudget(plan, diffs, 10000)
    expect(chunks[0][0].file).toBe("critical.ts")
    expect(chunks[0][1].file).toBe("high.ts")
    expect(chunks[0][2].file).toBe("low.ts")
  })

  it("should handle empty plan", () => {
    const chunks = chunkByTokenBudget([], {}, 1000)
    expect(chunks).toEqual([])
  })
})

describe("parseFindings", () => {
  it("should parse valid JSON array", () => {
    const text = JSON.stringify([
      { file: "a.ts", line: 10, category: "bug", severity: "warning", message: "test", confidence: 0.8 },
    ])
    const findings = parseFindings(text)
    expect(findings).toHaveLength(1)
    expect(findings[0].file).toBe("a.ts")
  })

  it("should extract JSON from markdown code blocks", () => {
    const text = '```json\n[{"file": "a.ts", "category": "bug", "severity": "warning", "message": "test", "confidence": 0.8}]\n```'
    const findings = parseFindings(text)
    expect(findings).toHaveLength(1)
  })

  it("should return empty array for invalid JSON", () => {
    const findings = parseFindings("not json at all")
    expect(findings).toEqual([])
  })

  it("should return empty array for empty array", () => {
    const findings = parseFindings("[]")
    expect(findings).toEqual([])
  })
})
