# Research Brief

## Question

Two-part investigation for Inno-Agent (Go backend, Mastra orchestrator, Ollama/vLLM LLM, Kafka queue, Docker deployment):

1. **LLM Latency Investigation**: Model latency has increased. Hypothesis: bottleneck is outside our codebase. Need to identify where latency appears across the entire pipeline, how to isolate each stage, what metrics to collect, and how to prove where the bottleneck actually is.

2. **Agent Planning Research**: Introduce planning into the agent layer — before executing complex tasks (PR review, code generation, refactoring), the agent creates a structured execution plan. Need to research how planning is implemented across major agent systems, deep dive into OpenHands and Mastra, and propose an architecture for Inno-Agent.

## Scope

**In scope:**
- End-to-end latency pipeline from GitHub PR webhook to review comment
- Every possible latency source (GPU, CPU, memory, network, queues, model internals)
- vLLM and Ollama performance characteristics and diagnostics
- Observability stack design (Prometheus, Grafana, OpenTelemetry)
- Agent planning architectures in: OpenHands, Claude Code, Codex, Cursor, Devin, Goose, Roo Code, Cline, Aider, Mastra, LangGraph, OpenAI Agents SDK, Anthropic Agents
- OpenHands deep dive (planning, execution, memory, task graph, sandbox, verification)
- Mastra capabilities assessment (planning, task graphs, workflows, reflection, checkpoints, memory)
- Architecture proposal tailored to Inno-Agent's existing stack
- MVP and production design recommendations

**Out of scope:**
- Model training or fine-tuning
- Cost optimization
- Security audit
- UI/UX design for the planning interface

## Assumptions

- Inno-Agent architecture: GitHub PR → Webhook → Kafka → Review Consumer → Orchestrator → Review Agent (Mastra) → LLM (Ollama/vLLM) → Review Comment
- Current LLM backend is transitioning from Ollama to vLLM
- Monitoring via Prometheus + Grafana already exists (may need expansion)
- Team is comfortable with Go and TypeScript (Mastra is TS-based)
- Planning should work within Mastra's existing agent framework if possible

## Depth mode

**deep** — 5-8 sub-agents, up to 2 follow-up rounds, 25+ sources target.

## Date

2026-07-25
