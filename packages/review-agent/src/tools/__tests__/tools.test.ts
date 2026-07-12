import { describe, it, expect, beforeEach, vi } from "vitest"
import { listChangedFiles } from "../list-changed-files"
import { getPrDiff } from "../get-pr-diff"
import { readRepositoryFile } from "../read-repository-file"
import { getPrComments } from "../get-pr-comments"

// Mock GitFlame client
vi.mock("../../services/gitflame-singleton", () => ({
  getGitFlameClient: () => ({
    listPRFiles: vi.fn().mockResolvedValue(["src/main.ts", "README.md", "package.json"]),
    getFileDiff: vi.fn().mockImplementation((_owner: string, _repo: string, _pullNumber: number, filename: string) => {
      if (filename === "src/main.ts") {
        return Promise.resolve("@@ -1,5 +1,6 @@\n import { something } from './lib'\n \n-function processOrder() {\n+function processOrder(order: Order) {\n+  validateOrder(order)\n   return order\n }")
      }
      return Promise.resolve("")
    }),
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

describe("listChangedFiles", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("should return list of changed files", async () => {
    const result = await listChangedFiles.execute({ owner: "test", repo: "repo", pullNumber: 1 }, { toolCallId: "test" }) as { files: string[] }
    expect(result.files).toEqual(["src/main.ts", "README.md", "package.json"])
  })
})

describe("getPrDiff", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("should return diff for a file", async () => {
    const result = await getPrDiff.execute({ owner: "test", repo: "repo", pullNumber: 1, filePath: "src/main.ts" }, { toolCallId: "test" }) as { diff: string }
    expect(result.diff).toContain("processOrder")
    expect(result.diff).toContain("validateOrder")
  })

  it("should return empty diff for unknown file", async () => {
    const result = await getPrDiff.execute({ owner: "test", repo: "repo", pullNumber: 1, filePath: "unknown.ts" }, { toolCallId: "test" }) as { diff: string }
    expect(result.diff).toBe("")
  })
})

describe("readRepositoryFile", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("should return file content when found", async () => {
    const result = await readRepositoryFile.execute({ owner: "test", repo: "repo", filePath: "AGENTS.md", ref: "abc123" }, { toolCallId: "test" }) as { found: boolean; content: string }
    expect(result.found).toBe(true)
    expect(result.content).toContain("AGENTS.md")
  })

  it("should return not found for missing file", async () => {
    const result = await readRepositoryFile.execute({ owner: "test", repo: "repo", filePath: "nonexistent.txt", ref: "abc123" }, { toolCallId: "test" }) as { found: boolean; content: string }
    expect(result.found).toBe(false)
    expect(result.content).toBe("")
  })

  it("should block non-allowed paths", async () => {
    const result = await readRepositoryFile.execute({ owner: "test", repo: "repo", filePath: "secrets.env", ref: "abc123" }, { toolCallId: "test" }) as { found: boolean; content: string }
    expect(result.found).toBe(false)
  })
})

describe("getPrComments", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("should return comments", async () => {
    const result = await getPrComments.execute({ owner: "test", repo: "repo", pullNumber: 1 }, { toolCallId: "test" }) as { comments: Array<{ body: string; author: string }> }
    expect(result.comments).toHaveLength(2)
    expect(result.comments[0].author).toBe("reviewer1")
  })
})
