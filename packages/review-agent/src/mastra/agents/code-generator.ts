import { Agent } from "@mastra/core/agent"
import { orchestratorModel } from "../model"
import { readSandboxFile } from "../../tools/read-sandbox-file"
import { writeSandboxFile } from "../../tools/write-sandbox-file"
import { searchCode } from "../../tools/search-code"
import { runCommand } from "../../tools/run-command"
import { runBuild } from "../../tools/run-build"
import { runTests } from "../../tools/run-tests"
import { runLint } from "../../tools/run-lint"

// SECURITY / AUDIT NOTE — attribution through orchestrator.
//
// The code-generator agent calls the orchestrator's OpenAI-compatible
// /v1/chat/completions endpoint with the issue author's delegated token
// as the bearer (see orchestratorModel in model.ts). The token arrives
// via RequestContext from the incoming request; if absent, the resolver
// throws rather than proceeding with service attribution. This gives
// per-user attribution/quota on the LLM call itself.
const codegenModel = process.env.CODEGEN_MODEL || process.env.REVIEW_MODEL || "qwen2.5-coder:1.5b"

// The agent edits a real repository tree that the workflow has populated into an
// isolated sandbox. It reads files, writes its changes with writeSandboxFile,
// and checks itself with the build/test tools — it does NOT print a JSON blob.
// The workflow collects the resulting changes via git diff, not from the reply.
const codegenInstructions = `You are a senior software engineer implementing GitFlame issues.

You are working inside a sandbox that already contains the repository's files.
Use your tools to do the work:
- read_sandbox_file / search_code — understand the existing code before changing it.
- write_sandbox_file — write your changes to files (full file contents, relative paths).
- run_build / run_tests / run_lint / run_command — check that your changes compile and pass.

Rules:
- Make the minimal change that resolves the issue. Do not rewrite unrelated code.
- Prefer editing existing files over creating new ones when the issue fits existing structure.
- After writing, build and test. If it fails, read the error and fix it.
- When done, reply with a short plain-text summary of what you changed and why. Do NOT
  paste file contents into the reply — the files are already written in the sandbox.`

// Code generator agent. Tools operate on the populated sandbox workspace, keyed
// per-run via RequestContext (see sandbox-run.ts). Build/test verification
// happens during generation now, not after push.
export const codeGeneratorAgent = new Agent({
  id: "code-generator",
  name: "Code Generator Agent",
  instructions: codegenInstructions,
  model: orchestratorModel(codegenModel),
  tools: {
    readSandboxFile,
    searchCode,
    writeSandboxFile,
    runCommand,
    runBuild,
    runTests,
    runLint,
  },
})
