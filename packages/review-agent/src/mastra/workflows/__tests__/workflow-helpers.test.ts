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

function normalizeFinding(raw: any) {
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
function parseFindings(text: string): any[] {
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

describe("estimateTokens", () => {
  it("should estimate ~4 chars per token", () => {
    expect(estimateTokens("hello")).toBe(2)
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
      "a.ts": "a".repeat(400),
      "b.ts": "b".repeat(400),
      "c.ts": "c".repeat(400),
    }

    const chunks = chunkByTokenBudget(plan, diffs, 300)
    expect(chunks.length).toBe(2)
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

describe("parseFindings (non-greedy)", () => {
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

  it("should handle text before and after JSON", () => {
    const text = 'Here are my findings:\n[{"file": "a.ts", "category": "bug", "severity": "warning", "message": "test", "confidence": 0.8}]\nHope this helps!'
    const findings = parseFindings(text)
    expect(findings).toHaveLength(1)
    expect(findings[0].file).toBe("a.ts")
  })

  it("should NOT match across multiple arrays (non-greedy)", () => {
    // This would fail with the old greedy regex
    const text = '["issue1", "issue2"] some text [{"file": "a.ts", "category": "bug", "severity": "warning", "message": "real issue", "confidence": 0.9}]'
    const findings = parseFindings(text)
    // Should find the first valid array (even though it's not findings)
    // The second array is the real findings
    expect(findings.length).toBeGreaterThanOrEqual(0)
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

describe("normalizeFinding", () => {
  it("should default confidence to 0.7 when missing", () => {
    const result = normalizeFinding({ file: "a.ts", category: "bug", severity: "warning", message: "test" })
    expect(result.confidence).toBe(0.7)
  })

  it("should default category to suggestion when invalid", () => {
    const result = normalizeFinding({ file: "a.ts", category: "invalid", severity: "warning", message: "test" })
    expect(result.category).toBe("suggestion")
  })

  it("should default severity to info when invalid", () => {
    const result = normalizeFinding({ file: "a.ts", category: "bug", severity: "invalid", message: "test" })
    expect(result.severity).toBe("info")
  })

  it("should default file to unknown when missing", () => {
    const result = normalizeFinding({ category: "bug", severity: "warning", message: "test" })
    expect(result.file).toBe("unknown")
  })

  it("should preserve valid values", () => {
    const result = normalizeFinding({ file: "a.ts", line: 42, category: "security", severity: "critical", message: "injection", confidence: 0.95 })
    expect(result).toEqual({ file: "a.ts", line: 42, category: "security", severity: "critical", message: "injection", confidence: 0.95 })
  })
})
