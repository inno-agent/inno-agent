import { describe, it, expect } from "vitest"
import { gzipSync } from "node:zlib"
import { parseLLMOutput } from "../codegen-pipeline"

// Mirrors the Go issue-consumer internal/generator/parse_test.go cases to
// verify the TypeScript port stays behaviourally identical.

describe("parseLLMOutput", () => {
  it("parses raw JSON with plain content", () => {
    const raw = `{"summary":"done","files":[{"path":"a.go","content":"package main"}]}`
    const result = parseLLMOutput(raw)
    expect(result).not.toBeNull()
    expect(result!.files).toHaveLength(1)
    expect(result!.files[0].path).toBe("a.go")
    expect(result!.files[0].content).toBe("package main")
  })

  it("parses JSON inside a markdown fence", () => {
    const raw = "Here is the result:\n```json\n{\"summary\":\"ok\",\"files\":[{\"path\":\"b.go\",\"content\":\"x\"}]}\n```"
    const result = parseLLMOutput(raw)
    expect(result).not.toBeNull()
    expect(result!.files).toHaveLength(1)
    expect(result!.files[0].path).toBe("b.go")
  })

  it("decodes base64 content", () => {
    const encoded = Buffer.from("print('hi')\n").toString("base64")
    const raw = `{"summary":"ok","files":[{"path":"main.py","content_base64":"${encoded}"}]}`
    const result = parseLLMOutput(raw)
    expect(result).not.toBeNull()
    expect(result!.files[0].content).toBe("print('hi')\n")
  })

  it("strips trailing commas", () => {
    const raw = `{"summary":"ok","files":[{"path":"a.go","content":"x",},],}`
    const result = parseLLMOutput(raw)
    expect(result).not.toBeNull()
    expect(result!.files).toHaveLength(1)
  })

  it("falls back to markdown fences when no JSON", () => {
    const raw = "Here you go:\n```python\nprint('hello')\n```\n"
    const result = parseLLMOutput(raw)
    expect(result).not.toBeNull()
    expect(result!.files).toHaveLength(1)
    expect(result!.files[0].path).toBe("main.py")
    expect(result!.files[0].content).toBe("print('hello')")
  })

  it("returns null for unparseable output", () => {
    const result = parseLLMOutput("just some text with no structure at all")
    expect(result).toBeNull()
  })

  it("decompresses gzip'd base64 content (model ignored no-gzip instruction)", () => {
    const gzipped = gzipSync(Buffer.from("print('gzip hi')\n"))
    const encoded = gzipped.toString("base64")
    const raw = `{"summary":"ok","files":[{"path":"main.py","content_base64":"${encoded}"}]}`
    const result = parseLLMOutput(raw)
    expect(result).not.toBeNull()
    expect(result!.files[0].content).toBe("print('gzip hi')\n")
  })
})
