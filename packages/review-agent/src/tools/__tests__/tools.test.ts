import { describe, it, expect, beforeEach, vi } from "vitest"
import { readRepositoryFile } from "../read-repository-file"
import { getPrComments } from "../get-pr-comments"

// Mock GitFlame client
vi.mock("../../services/gitflame-singleton", () => ({
  getGitFlameClient: () => ({
    getRawFile: vi.fn().mockImplementation((_owner: string, _repo: string, path: string) => {
      if (path === "AGENTS.md") {
        return Promise.resolve({ content: "# AGENTS.md\nTest guidelines", found: true })
      }
      if (path === "README.md") {
        return Promise.resolve({ content: "# Project\nTest project", found: true })
      }
      if (path === "secrets.env") {
        return Promise.resolve({ content: "SECRET=abc", found: true })
      }
      return Promise.resolve({ content: "", found: false })
    }),
    getPRComments: vi.fn().mockResolvedValue([
      { body: "Looks good!", author: "reviewer1" },
      { body: "Please fix the typo", author: "reviewer2" },
    ]),
  }),
}))

const ctx = {} as any

describe("readRepositoryFile", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("should return file content when found", async () => {
    const result = await readRepositoryFile.execute!({ owner: "test", repo: "repo", filePath: "AGENTS.md", ref: "abc123" }, ctx) as { found: boolean; content: string }
    expect(result.found).toBe(true)
    expect(result.content).toContain("AGENTS.md")
  })

  it("should return not found for missing file", async () => {
    const result = await readRepositoryFile.execute!({ owner: "test", repo: "repo", filePath: "nonexistent.txt", ref: "abc123" }, ctx) as { found: boolean; content: string }
    expect(result.found).toBe(false)
    expect(result.content).toBe("")
  })

  it("should block non-allowed paths", async () => {
    const result = await readRepositoryFile.execute!({ owner: "test", repo: "repo", filePath: "secrets.env", ref: "abc123" }, ctx) as { found: boolean; content: string }
    expect(result.found).toBe(false)
  })
})

describe("getPrComments", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("should return comments", async () => {
    const result = await getPrComments.execute!({ owner: "test", repo: "repo", pullNumber: 1 }, ctx) as { comments: Array<{ body: string; author: string }> }
    expect(result.comments).toHaveLength(2)
    expect(result.comments[0].author).toBe("reviewer1")
  })
})
