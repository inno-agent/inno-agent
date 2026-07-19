import { Agent } from "@mastra/core/agent"

// SECURITY / AUDIT NOTE — token model differs from the single-shot fallback.
//
// The code-generator agent talks to Ollama's OpenAI-compatible API directly,
// same as code-reviewer (the orchestrator exposes /v1/chat, not
// /v1/chat/completions, so Mastra's OpenAI client 404s against it). Direct
// Ollama calls carry NO per-user token: Ollama has no auth concept, and this
// call never goes through the orchestrator's RFC 8693 delegated-token flow.
//
// This is a deliberate difference from issue-consumer's single-shot fallback
// (backend/issue-consumer/internal/generator/service.go), which exchanges the
// issue assigner's identity for a delegated user token via
// backend/issue-consumer/internal/tokensource and attaches it to every
// orchestrator /v1/chat call — giving per-user attribution/quota on the LLM
// call itself. When CODEGEN_AGENT_URL is set (this file is in use), that
// attribution does not happen: this service authenticates to GitFlame with
// its own static GITFLAME_TOKEN (see gitflame-singleton.ts) and to the model
// with no token at all. If per-user LLM attribution/quota is required for
// codegen, route this agent through the orchestrator instead of Ollama
// directly, and thread a token from the request through to it — do not
// assume the single-shot path's guarantees hold here.
const ollamaUrl = process.env.OLLAMA_BASE_URL || "http://ollama:11434"
const codegenModel = process.env.CODEGEN_MODEL || process.env.REVIEW_MODEL || "qwen2.5-coder:1.5b"

const modelUrl = `${ollamaUrl.replace(/\/$/, "")}/v1`

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
  model: {
    id: `custom/${codegenModel}`,
    url: modelUrl,
  },
})
