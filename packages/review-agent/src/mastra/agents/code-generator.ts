import { Agent } from "@mastra/core/agent"
import { orchestratorModel } from "../model"

// SECURITY / AUDIT NOTE — attribution through orchestrator.
//
// The code-generator agent calls the orchestrator's OpenAI-compatible
// /v1/chat/completions endpoint with the issue author's delegated token
// as the bearer (see orchestratorModel in model.ts). The token arrives
// via RequestContext from the incoming request; if absent, the resolver
// throws rather than proceeding with service attribution. This gives
// per-user attribution/quota on the LLM call itself, matching the single-shot
// fallback (backend/issue-consumer/internal/generator/service.go).
const codegenModel = process.env.CODEGEN_MODEL || process.env.REVIEW_MODEL || "qwen2.5-coder:1.5b"

// System prompt mirrors the Go issue-consumer generator.codegenSystemPrompt
// so behaviour is identical whether the consumer uses Mastra or single-shot.
const codegenInstructions = `You are a senior software engineer implementing GitFlame issues.

Return ONLY a single JSON object. No markdown, no code fences, no explanation.

Schema:
{"summary":"what you implemented","files":[{"path":"relative/path","content_base64":"BASE64_UTF8"}]}

Example for a Python script in main.py:
{"summary":"add two numbers","files":[{"path":"main.py","content_base64":"YSA9IGludChpbnB1dCgpCmIgPSBpbnQoaW5wdXQoKSkKcHJpbnQoYSArIGIp"}]}

Rules:
- content_base64 must be standard base64 of the full UTF-8 file (one line, no spaces). Do not gzip or compress.
- Include complete files, not diffs.
- Minimal changes only.`

// Code generator agent. Tools are intentionally omitted: the agent produces
// files from the issue description + repo context (AGENTS.md/README.md) that
// the workflow fetches for it. Build/test verification happens after the files
// are pushed to a branch, not during generation.
export const codeGeneratorAgent = new Agent({
  id: "code-generator",
  name: "Code Generator Agent",
  instructions: codegenInstructions,
  model: orchestratorModel(codegenModel),
})
