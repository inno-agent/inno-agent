import { createWorkflow, createStep } from "@mastra/core/workflows"
import { z } from "zod"
import { gunzipSync } from "node:zlib"
import { getGitFlameClient } from "../../services/gitflame-singleton"
import { codeGeneratorAgent } from "../agents/code-generator"

// ─── Schemas ────────────────────────────────────────────────────────────────

const CodegenInputSchema = z.object({
  owner: z.string(),
  repo: z.string(),
  issueNumber: z.number(),
  defaultBranch: z.string().optional(),
  issueType: z.string().optional(),
  // Optional: the webhook already carries title/body. When empty, the workflow
  // fetches them from GitFlame.
  title: z.string().optional(),
  body: z.string().optional(),
})

const GeneratedFileSchema = z.object({
  path: z.string(),
  content: z.string(),
})

const CodegenOutputSchema = z.object({
  summary: z.string(),
  files: z.array(GeneratedFileSchema),
})

// ─── Prompts ────────────────────────────────────────────────────────────────

const codegenRepairPrompt = `Your reply was not usable. Return ONLY one JSON object.
Do not use markdown fences or comments. Every file must have path and content_base64.
content_base64 must be valid standard base64 of raw UTF-8 file bytes (not gzip).
Example: {"summary":"done","files":[{"path":"main.py","content_base64":"cHJpbnQoMSk="}]}`

// ─── Step 1: fetchIssueContext (deterministic, no LLM) ──────────────────────

const fetchIssueContextStep = createStep({
  id: "fetch-issue-context",
  inputSchema: CodegenInputSchema,
  outputSchema: z.object({
    owner: z.string(),
    repo: z.string(),
    issueNumber: z.number(),
    defaultBranch: z.string(),
    issueType: z.string(),
    title: z.string(),
    body: z.string(),
    agentsMd: z.string(),
    readmeMd: z.string(),
  }),
  execute: async ({ inputData }) => {
    const { owner, repo, issueNumber, defaultBranch, issueType } = inputData
    const client = getGitFlameClient()
    const ref = defaultBranch || "main"

    let title = inputData.title || ""
    let body = inputData.body || ""

    // Fetch the issue when the webhook payload is missing EITHER field. The
    // common webhook shape carries a title but no body; requiring both to be
    // absent before fetching left the generator working from a title alone.
    if (!title || !body) {
      try {
        const issue = await client.getIssue(owner, repo, issueNumber)
        if (issue.title) title = issue.title
        if (issue.body) body = issue.body
      } catch (err) {
        console.warn(`[codegen] failed to fetch issue ${owner}/${repo}#${issueNumber}:`, err)
      }
    }

    let agentsMd = "(absent)"
    try {
      const result = await client.getRawFile(owner, repo, "AGENTS.md", ref)
      if (result.found) agentsMd = result.content
    } catch (err) {
      console.warn("[codegen] failed to fetch AGENTS.md:", err)
    }

    let readmeMd = "(absent)"
    try {
      const result = await client.getRawFile(owner, repo, "README.md", ref)
      if (result.found) readmeMd = result.content
    } catch (err) {
      console.warn("[codegen] failed to fetch README.md:", err)
    }

    return {
      owner,
      repo,
      issueNumber,
      defaultBranch: ref,
      issueType: issueType || "issue",
      title,
      body,
      agentsMd,
      readmeMd,
    }
  },
})

// ─── Step 2: generate (LLM call + parse with repair retries) ────────────────

const generateStep = createStep({
  id: "generate",
  inputSchema: z.object({
    owner: z.string(),
    repo: z.string(),
    issueNumber: z.number(),
    defaultBranch: z.string(),
    issueType: z.string(),
    title: z.string(),
    body: z.string(),
    agentsMd: z.string(),
    readmeMd: z.string(),
  }),
  outputSchema: CodegenOutputSchema,
  execute: async ({ inputData }) => {
    const { owner, repo, defaultBranch, issueType, issueNumber, title, body, agentsMd, readmeMd } = inputData

    const userMsg = `Repository: ${owner}/${repo}\nDefault branch: ${defaultBranch}\nIssue type: ${issueType}\nIssue #${issueNumber}\nTitle: ${title}\n\nDescription:\n${body}\n\n=== AGENTS.md ===\n${agentsMd}\n\n=== README.md ===\n${readmeMd}`

    const start = Date.now()
    let raw = await codeGeneratorAgent.generate(userMsg)
    console.log(`[codegen] generate completed in ${Date.now() - start}ms`)

    let result = parseLLMOutput(raw.text)
    for (let attempt = 0; attempt < 2 && !result; attempt++) {
      console.warn(`[codegen] parse failed; requesting repair (attempt ${attempt + 1})`)
      raw = await codeGeneratorAgent.generate(`${userMsg}\n\n--- Previous reply ---\n${raw.text}\n\n--- ${codegenRepairPrompt}`)
      result = parseLLMOutput(raw.text)
    }

    if (!result) {
      throw new Error("codegen: failed to parse LLM output after repair retries")
    }
    if (result.files.length === 0) {
      throw new Error("codegen: LLM returned no files")
    }
    return result
  },
})

// ─── Workflow ───────────────────────────────────────────────────────────────

export const codegenPipeline = createWorkflow({
  id: "codegen-pipeline",
  inputSchema: CodegenInputSchema,
  outputSchema: CodegenOutputSchema,
})
  .then(fetchIssueContextStep)
  .then(generateStep)
  .commit()

// ─── LLM output parser (ported from Go issue-consumer internal/generator/parse.go) ──

export interface GeneratedFile {
  path: string
  content: string
}

export interface GenerationResult {
  summary: string
  files: GeneratedFile[]
}

const jsonFenceRE = /```(?:json)?\s*([\s\S]*?)```/g
const anyFenceRE = /```([a-zA-Z0-9._-]*)\s*\n([\s\S]*?)```/g
const trailingCommaRE = /,(\s*[}\]])/g

interface LLMFile {
  path?: string
  content?: string
  content_base64?: string
}

interface LLMOutput {
  summary?: string
  files?: LLMFile[]
}

interface FencedBlock {
  lang: string
  content: string
}

export function parseLLMOutput(raw: string): GenerationResult | null {
  const candidates = jsonCandidates(raw)
  let lastErr: Error | null = null

  for (const jsonText of candidates) {
    try {
      return parseJSONObject(jsonText, raw)
    } catch (err) {
      lastErr = err as Error
      continue
    }
  }

  try {
    return parseFromMarkdownFences(raw)
  } catch (err) {
    if (lastErr === null) lastErr = err as Error
  }

  if (lastErr && lastErr.message.includes("no code in markdown fences")) {
    lastErr = new Error("json found but file contents were missing or invalid base64")
  }
  return null
}

function parseJSONObject(jsonText: string, raw: string): GenerationResult {
  jsonText = sanitizeJSONObject(jsonText)

  let out: LLMOutput
  try {
    out = JSON.parse(jsonText)
  } catch (err) {
    throw err
  }

  const files = filesFromOutput(out, raw)
  if (files.length === 0) throw new Error("no files in output")

  return {
    files,
    summary: (out.summary || "").trim(),
  }
}

function jsonCandidates(raw: string): string[] {
  const trimmed = raw.trim()
  const ordered: string[] = []
  const seen = new Set<string>()

  const add = (s: string) => {
    s = s.trim()
    if (!s || seen.has(s)) return
    seen.add(s)
    ordered.push(s)

    const cleaned = sanitizeJSONObject(s)
    if (cleaned !== s && cleaned && !seen.has(cleaned)) {
      seen.add(cleaned)
      ordered.push(cleaned)
    }
  }

  add(trimmed)
  const obj = extractJSONObject(trimmed)
  if (obj) add(obj)

  // Reset regex stateful lastIndex by creating new matches via matchAll.
  for (const m of trimmed.matchAll(jsonFenceRE)) {
    if (m[1]) {
      add(m[1])
      const inner = extractJSONObject(m[1])
      if (inner) add(inner)
    }
  }

  for (const m of trimmed.matchAll(anyFenceRE)) {
    if (m[2] === undefined) continue
    const body = m[2].trim()
    add(body)
    const inner = extractJSONObject(body)
    if (inner) add(inner)
  }

  return ordered
}

function extractJSONObject(s: string): string {
  const start = s.indexOf("{")
  const end = s.lastIndexOf("}")
  if (start < 0 || end <= start) return ""
  return s.slice(start, end + 1)
}

function sanitizeJSONObject(s: string): string {
  s = stripJSONComments(s)
  s = s.replace(trailingCommaRE, "$1")
  return s.trim()
}

function stripJSONComments(s: string): string {
  let out = ""
  let inString = false
  let escaped = false

  for (let i = 0; i < s.length; i++) {
    const c = s[i]

    if (inString) {
      out += c
      if (escaped) {
        escaped = false
        continue
      }
      if (c === "\\") {
        escaped = true
        continue
      }
      if (c === '"') inString = false
      continue
    }

    if (c === '"') {
      inString = true
      out += c
      continue
    }

    if (c === "/" && i + 1 < s.length) {
      if (s[i + 1] === "/") {
        i += 2
        while (i < s.length && s[i] !== "\n") i++
        if (i < s.length) out += "\n"
        continue
      }
      if (s[i + 1] === "*") {
        i += 2
        while (i + 1 < s.length && !(s[i] === "*" && s[i + 1] === "/")) i++
        if (i + 1 < s.length) i++
        continue
      }
    }

    out += c
  }

  return out
}

function filesFromOutput(out: LLMOutput, raw: string): GeneratedFile[] {
  const fences = extractFencedBlocks(raw)
  const contentFences = filterContentFences(fences)

  const files: GeneratedFile[] = []
  let fenceIdx = 0

  for (const f of out.files || []) {
    const path = (f.path || "").trim()
    if (!path) continue

    let content = decodeFileContent(f)
    if (!content) {
      if (fenceIdx < contentFences.length && (!looksLikeJSON(contentFences[fenceIdx].content) || hasJSONExtension(path))) {
        content = contentFences[fenceIdx].content.trim()
        fenceIdx++
      }
    }
    if (!content) {
      throw new Error(`file ${path} declared but has no content (not in JSON and not in any markdown fence)`)
    }

    files.push({ path, content })
  }

  if (files.length === 0 && fences.length > 0) {
    return filesFromFences(fences)
  }

  return files
}

function hasJSONExtension(path: string): boolean {
  return path.endsWith(".json")
}

function isMainSpecFence(content: string): boolean {
  if (!content.includes("summary")) return false
  if (!content.includes("files")) return false
  try {
    const obj = JSON.parse(content)
    return obj && typeof obj === "object" && "summary" in obj && "files" in obj
  } catch {
    return false
  }
}

function filterContentFences(fences: FencedBlock[]): FencedBlock[] {
  const result: FencedBlock[] = []
  for (const f of fences) {
    if (looksLikeJSON(f.content) && isMainSpecFence(f.content.trim())) continue
    result.push(f)
  }
  return result
}

function decodeFileContent(f: LLMFile): string {
  const enc = (f.content_base64 || "").trim()
  if (enc) {
    const decoded = decodeBase64Lenient(enc)
    if (decoded) {
      const text = decodeFileBytes(decoded)
      if (text) return text
    }
  }
  const content = (f.content || "").trim()
  if (content) return content
  return ""
}

function decodeFileBytes(raw: Uint8Array): string {
  // Gzip magic bytes → decompress. Mirrors the Go decodeFileBytes fallback
  // (issue-consumer internal/generator/parse.go), which some models trigger
  // by gzip-compressing content_base64 despite instructions not to.
  if (raw.length >= 2 && raw[0] === 0x1f && raw[1] === 0x8b) {
    try {
      const decompressed = gunzipSync(Buffer.from(raw))
      return decompressed.toString("utf-8")
    } catch {
      return ""
    }
  }
  return new TextDecoder("utf-8").decode(raw)
}

const base64StdRE = /^[A-Za-z0-9+/]+$/
const base64UrlRE = /^[A-Za-z0-9\-_]+$/

// decodeBase64Lenient decodes standard or URL-safe base64, tolerating missing
// padding and embedded whitespace, and returns null for anything else.
//
// The validation is not optional: Node's Buffer.from(s, "base64") NEVER throws,
// it silently drops characters outside the alphabet. Without an explicit check,
// a model that puts plain prose in content_base64 decodes to garbage bytes,
// which then look like a successful decode and get written to the branch —
// the `content` fallback in decodeFileContent would never be reached. Go's
// base64.StdEncoding.DecodeString errors on invalid input; this mirrors that.
function decodeBase64Lenient(enc: string): Uint8Array | null {
  enc = enc.replace(/[\n\r\s\t]/g, "")
  if (!enc) return null

  const stripped = enc.replace(/=+$/, "")
  const isStd = base64StdRE.test(stripped)
  const isUrl = !isStd && base64UrlRE.test(stripped)
  if (!isStd && !isUrl) return null

  // base64 encodes in 4-char groups; a remainder of 1 char is never valid.
  if (stripped.length % 4 === 1) return null

  const buf = Buffer.from(stripped, isUrl ? "base64url" : "base64")
  if (buf.length === 0) return null
  return new Uint8Array(buf)
}

function parseFromMarkdownFences(raw: string): GenerationResult {
  const fences = extractFencedBlocks(raw)
  if (fences.length === 0) throw new Error("no markdown fences found")

  const files = filesFromFences(fences)
  return {
    files,
    summary: "Generated from markdown code block",
  }
}

function filesFromFences(fences: FencedBlock[]): GeneratedFile[] {
  const files: GeneratedFile[] = []
  const usedPaths = new Map<string, number>()

  fences.forEach((fb, i) => {
    const content = fb.content.trim()
    if (!content || looksLikeJSON(content)) return

    let path = defaultPathForLang(fb.lang, i)
    const n = usedPaths.get(path) || 0
    if (n > 0) {
      const ext = pathExtension(path)
      const base = path.slice(0, path.length - ext.length)
      path = `${base}_${n + 1}${ext}`
    }
    usedPaths.set(path, n + 1)

    files.push({ path, content })
  })

  if (files.length === 0) throw new Error("no code in markdown fences")
  return files
}

function extractFencedBlocks(raw: string): FencedBlock[] {
  const blocks: FencedBlock[] = []
  for (const m of raw.matchAll(anyFenceRE)) {
    if (m[2] === undefined) continue
    blocks.push({
      lang: (m[1] || "").trim().toLowerCase(),
      content: m[2],
    })
  }
  return blocks
}

function looksLikeJSON(s: string): boolean {
  s = s.trim()
  return s.startsWith("{") || s.startsWith("[")
}

function defaultPathForLang(lang: string, index: number): string {
  switch (lang) {
    case "python":
    case "py":
      return index === 0 ? "main.py" : `main_${index + 1}.py`
    case "go":
    case "golang":
      return "main.go"
    case "javascript":
    case "js":
      return "index.js"
    case "typescript":
    case "ts":
      return "index.ts"
    case "java":
      return "Main.java"
    case "rust":
    case "rs":
      return "main.rs"
    case "sh":
    case "bash":
      return "script.sh"
    default:
      return index === 0 ? "solution.txt" : `solution_${index + 1}.txt`
  }
}

function pathExtension(path: string): string {
  const i = path.lastIndexOf(".")
  return i >= 0 ? path.slice(i) : ""
}
