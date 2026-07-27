import { describe, it, expect } from "vitest"
import { syntaxCheckCommand } from "../review-pipeline"

describe("syntaxCheckCommand", () => {
  it("uses py_compile for .py", () => {
    const cmd = syntaxCheckCommand("app/main.py")
    expect(cmd).toContain("python3 -m py_compile")
    expect(cmd).toContain("[ -f 'app/main.py' ]") // guarded against missing file
  })

  it("uses node --check for .js/.mjs/.cjs", () => {
    expect(syntaxCheckCommand("a.js")).toContain("node --check")
    expect(syntaxCheckCommand("a.mjs")).toContain("node --check")
    expect(syntaxCheckCommand("a.cjs")).toContain("node --check")
  })

  it("uses gofmt -e for .go", () => {
    expect(syntaxCheckCommand("pkg/x.go")).toContain("gofmt -e")
  })

  it("returns null for unsupported extensions", () => {
    expect(syntaxCheckCommand("README.md")).toBeNull()
    expect(syntaxCheckCommand("data.json")).toBeNull()
    expect(syntaxCheckCommand("noext")).toBeNull()
  })

  it("escapes single quotes to prevent shell injection", () => {
    const cmd = syntaxCheckCommand("a'; rm -rf /; '.py")
    expect(cmd).not.toBeNull()
    // the malicious quote must be escaped, not left to break out of the quoting
    expect(cmd).toContain("'\\''")
  })
})
