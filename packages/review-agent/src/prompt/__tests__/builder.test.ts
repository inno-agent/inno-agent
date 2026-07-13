import { describe, it, expect } from "vitest"
import { readFileSync } from "fs"
import { join, dirname } from "path"
import { fileURLToPath } from "url"

const __dirname = dirname(fileURLToPath(import.meta.url))

describe("review.md prompt", () => {
  it("should exist and be non-empty", () => {
    const content = readFileSync(join(__dirname, "..", "review.md"), "utf-8")
    expect(content.length).toBeGreaterThan(100)
  })

  it("should contain output format instructions", () => {
    const content = readFileSync(join(__dirname, "..", "review.md"), "utf-8")
    expect(content).toContain("JSON")
    expect(content).toContain("category")
    expect(content).toContain("severity")
    expect(content).toContain("confidence")
  })

  it("should NOT reference tools (pure text gen workflow)", () => {
    const content = readFileSync(join(__dirname, "..", "review.md"), "utf-8")
    expect(content).not.toContain("read-repository-file")
    expect(content).not.toContain("get-pr-comments")
  })
})
