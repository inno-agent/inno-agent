# Inno-Agent: LLM Latency Investigation & Agent Planning Architecture

> Generated 2026-07-25 · depth: deep · 30+ sources · workspace: research/inno-agent-llm-latency-agent-planning/

---

## Executive Summary

**Latency Investigation:**
- The latency pipeline has 10+ stages; the most likely bottlenecks are **LLM inference** (prefill/decode), **Ollama queueing** (512-request queue with head-of-line blocking), and **tool-calling round-trips** (each tool call = full prefill+decode cycle) [1][2][3]
- vLLM provides 30+ native Prometheus metrics including TTFT, ITL, queue time, prefill time, decode time, KV cache usage, and prefix cache hit rates — no custom exporter needed [4][5]
- Ollama has **no native Prometheus metrics** and serializes model loading to one-at-a-time, making it fundamentally harder to monitor and slower under concurrency [3][6]
- Preemption events in vLLM (`vllm:num_preemptions`) are the single most expensive latency event — they trigger full re-prefill from scratch [4]
- Moving from Ollama to vLLM is the right call for production: vLLM's continuous batching, paged attention, and prefix caching directly address Ollama's concurrency limitations [4][7]

**Agent Planning:**
- Claude Code's **Plan/Act separation** (read-only planning subagent → execution in main context) is the clearest, most battle-tested planning architecture found [8]
- OpenHands' **Goal Completion Loop** (separate judge LLM verifying task completion) provides a strong verification model [9]
- Mastra supports workflows and agent loops but **lacks native planning, task graphs, and structured execution plans** — building a custom planning layer on top of Mastra is the recommended approach [10]
- The recommended architecture: **Planner → Execution Plan (DAG) → Step Executor → Reflection → Replan → Verifier** [11]
- MVP: Add a planning prompt + structured plan parser + step-by-step execution with reflection checks — ~2 weeks of engineering [11]

---

## Background & Scope

Inno-Agent is a PR review system: GitHub PR → Webhook → Kafka → Review Consumer → Orchestrator → Review Agent (Mastra) → LLM (Ollama/vLLM) → Review Comment. Model latency has increased, and the hypothesis is the bottleneck is outside our codebase. This report covers: (1) full latency pipeline diagnostics, (2) vLLM and Ollama performance characteristics, (3) observability stack design, (4) agent planning architectures across major systems, and (5) a concrete planning architecture for Inno-Agent.

---

# Part 1 — LLM Latency Investigation

## 1. End-to-End Latency Pipeline

Every stage from GitHub webhook to review comment, with expected latency, bottlenecks, and metrics:

```
GitHub PR Created/Updated
  ↓ [~100-500ms] Webhook delivery (GitHub → your server)
  ↓ [~1-5ms] Webhook validation + parsing
  ↓ [~1-10ms] Kafka produce (enqueue review request)
  ↓ [~10-100ms] Kafka consume (consumer group fetch)
  ↓ [~1-5ms] Orchestrator dispatch
  ↓ [~50-200ms] Agent initialization + prompt construction
  ↓ [~100-500ms] Tool calls (file read, AST parse, etc.)
  ↓ [~500ms-30s] LLM inference
  │   ├── [~50-2000ms] Queue wait (if vLLM busy)
  │   ├── [~100ms-5s] Prefill (scales with prompt length)
  │   ├── [~50-200ms] Time to First Token (TTFT)
  │   └── [~10-50ms/token] Decode (scales with output length)
  ↓ [~10-50ms] Tool calling round-trip (if multi-step)
  ↓ [~100-500ms] Response assembly + streaming
  ↓ [~50-200ms] GitHub API call (post review comment)
  ↓ [~10-50ms] GitHub rate limit check + write
```

### Stage Details

**Webhook Ingestion (100-500ms)**
- GitHub enforces a **10-second response timeout** for webhooks [1]
- GitHub recommends async queue architecture to avoid blocking [1]
- Webhook payloads capped at **25 MB** — large PRs may fail silently [1]
- **Metric**: `webhook_received_timestamp`, `webhook_processed_timestamp`

**Kafka Queue (1-100ms typical, seconds under load)**
- Produce-to-consume latency depends on batch size, compression, and consumer prefetch
- Consumer group rebalancing causes multi-second pauses
- **Metrics**: `kafka_consumer_lag`, `kafka_produce_latency_ms`, `kafka_fetch_latency_ms`

**Orchestrator (1-5ms)**
- Go-based dispatch; negligible unless blocked on downstream
- **Metric**: `orchestrator_dispatch_duration_ms`

**Agent Execution (50ms-30s, highly variable)**
- Prompt construction: template rendering + file content assembly
- Tool calling: **each tool call is a full LLM round-trip** (prefill + decode) [2]
- A 5-tool review = **5x LLM latency** [2]
- **Metrics**: `agent_execution_duration_ms`, `agent_tool_calls_count`, `agent_tool_call_duration_ms`

**LLM Inference (500ms-60s, dominant stage)**
- See detailed vLLM/Ollama sections below
- **Metrics**: `llm_ttft_ms`, `llm_itl_ms`, `llm_tokens_per_second`, `llm_queue_time_ms`

**GitHub Comment Posting (50-200ms)**
- Secondary rate limits cap content-creating requests at **80/min** [1]
- Under burst load, comment posting can be delayed by rate limiting
- **Metrics**: `github_api_duration_ms`, `github_api_rate_limit_remaining`

---

## 2. Latency Sources — Comprehensive Catalog

### GPU-Related

| Source | Symptoms | Verification | Metrics |
|--------|----------|-------------|---------|
| GPU utilization saturation | High ITL, low throughput | `nvidia-smi` shows >90% utilization | `DCGM_FI_DEV_GPU_UTIL` |
| GPU memory pressure | OOM kills, preemptions | `nvidia-smi` shows >95% memory | `DCGM_FI_DEV_FB_USED`, `vllm:kv_cache_usage_perc` |
| GPU thermal throttling | Gradual performance degradation | `nvidia-smi` shows high temperature | `DCGM_FI_DEV_GPU_TEMP` |
| Multi-GPU PCIe transfer | Higher latency than single-GPU | Compare single vs multi-GPU TTFT | `vllm:request_prefill_time_seconds` |
| GPU scheduling contention | Intermittent latency spikes | Check for other processes on GPU | `nvidia-smi` process list |

### CPU & Memory

| Source | Symptoms | Verification | Metrics |
|--------|----------|-------------|---------|
| CPU bottleneck in tokenization | High prefill, low GPU util | Profile tokenization separately | `process_cpu_seconds_total` |
| Go GC pauses | Periodic latency spikes in orchestrator | pprof heap profile, GC pause histogram | `go_gc_duration_seconds` |
| Node.js event loop blocking | Mastra agent stalls | `--prof` or clinic.js flame graph | `nodejs_eventloop_lag_p99` |
| OS memory pressure / swap | System-wide slowdown | `vmstat`, `dmesg` OOM | `node_memory_MemAvailable_bytes` |
| Docker container memory limits | OOM kills, restarts | `docker stats` | `container_memory_usage_bytes` |

### Queue & Network

| Source | Symptoms | Verification | Metrics |
|--------|----------|-------------|---------|
| Kafka consumer lag | Delayed review processing | Consumer lag metrics | `kafka_consumer_lag` |
| Kafka rebalancing | Multi-second pauses | Consumer group logs | `kafka_rebalance_total` |
| Docker bridge networking | Added latency between containers | Compare bridge vs host networking | `container_network_receive_bytes_total` |
| Reverse proxy overhead | Added 1-5ms per hop | Trace through proxy layers | `nginx_upstream_response_time` |
| HTTP connection pooling | Connection setup overhead | Check for new connections per request | `http_connections_total` |

### Model-Specific

| Source | Symptoms | Verification | Metrics |
|--------|----------|-------------|---------|
| Long prompt prefill | High TTFT, low ITL | Correlate TTFT with input length | `vllm:request_prefill_time_seconds` |
| KV cache exhaustion | Preemptions, queue buildup | Cache usage >90% | `vllm:kv_cache_usage_perc`, `vllm:num_preemptions` |
| Cold start (model load) | Seconds of delay on first request | Check model load time | `model_load_duration_seconds` |
| Ollama model serialization | One model load at a time | Source: sched.go [6] | Custom metric needed |
| Context length auto-scaling | Unexpected VRAM consumption | Check OLLAMA_CONTEXT_LENGTH | Custom metric needed |

---

## 3. vLLM Performance Deep Dive

### Prefill Phase

- Processes entire input prompt through all transformer layers to compute KV cache entries [4]
- **Dominates TTFT**; scales linearly with prompt length
- With 1000 input tokens, prefill is ~1000x slower than a single decode step [4]
- **Key metrics**: `vllm:request_prefill_time_seconds`, `vllm:time_to_first_token_seconds`, `vllm:request_prefill_kv_computed_tokens`
- **Diagnosis**: High TTFT + low ITL = prefill bottleneck. Check `kv_cache_usage_perc` near 1.0 [4]

### Decode Phase

- Generates tokens one at a time; each step runs full model forward but processes 1 new token [4]
- Determines ITL (Inter-Token Latency); scales with model size and batch size
- **Key metrics**: `vllm:request_decode_time_seconds`, `vllm:inter_token_latency_seconds`, `vllm:request_time_per_output_token_seconds`
- **Diagnosis**: High ITL = decode bottleneck. Check `num_requests_running` — more concurrent requests = higher ITL [4]

### Continuous Batching

- New requests join active batch between decode steps — no head-of-line blocking [4]
- Slow requests don't block others, but high batch occupancy increases ITL for all
- **Key metrics**: `vllm:num_requests_running`, `vllm:num_requests_waiting`, `vllm:request_queue_time_seconds`
- **Diagnosis**: High queue time = insufficient capacity. `waiting_by_reason` labels: 'capacity' vs 'deferred' [4]

### Paged Attention

- KV cache stored in fixed-size blocks (pages) on GPU, analogous to OS virtual memory [4]
- Eliminates memory fragmentation; enables efficient batching
- Default block size: 16 tokens [4]
- **Key metrics**: `vllm:kv_cache_usage_perc`, `vllm:kv_block_lifetime_seconds`, `vllm:kv_block_idle_before_evict_seconds`, `vllm:kv_block_reuse_gap_seconds`
- **Diagnosis**: `kv_cache_usage_perc` > 0.9 → high preemption risk. High block lifetime = requests holding cache too long [4]

### Automatic Prefix Caching (APC)

- Caches KV blocks so new requests sharing the same prefix skip re-computation [4]
- Uses SHA-256 block hashing (v0.11+); only caches full blocks (default 16 tokens)
- **Only helps prefill, not decode** [4]
- **Key metrics**: `vllm:prefix_cache_queries`, `vllm:prefix_cache_hits`
- **Hit rate**: `rate(cache_hits[5m]) / rate(cache_queries[5m])`
- **Configuration**: `--enable-prefix-caching` (default: enabled in V1), `--prefix-caching-hash-algo` (sha256, xxhash)

### Speculative Decoding

- Draft model proposes candidate tokens; target model validates in parallel [4]
- Lossless output; most effective at low-to-medium QPS
- **Methods**: EAGLE (general), MTP (native support), Draft Model, N-gram, Suffix Decoding
- **Metrics**: `vllm:spec_decode_num_accepted_tokens_per_pos`
- **Caveats**: Pipeline parallelism incompatible (v≤0.15.0). Draft model reduces KV cache for main model [4]

### Chunked Prefill

- Splits long prefill across multiple forward passes, interleaving with decode [4]
- Improves ITL stability under mixed workloads
- Trades slightly higher TTFT for much lower ITL variance
- **Metrics**: `vllm:iteration_tokens_total` (reveals batch mixing)

### Preemption

- **Most expensive latency event** — causes full re-prefill from scratch [4]
- Triggered by KV cache exhaustion
- **Metric**: `vllm:num_preemptions` (Counter)
- **Diagnosis**: Any non-zero value means latency spikes. Increase `gpu_memory_utilization` or reduce `max_model_len`

---

## 4. Ollama Latency Characteristics

### Queueing Behavior

- Default queue size: **512 requests** before HTTP 503 rejection [3][6]
- **Serializes model loading** to one at a time — only one model load can occur concurrently [6]
- Requests to already-loaded models run in parallel [6]
- Single-goroutine scheduler (`processPending`) means **one slow scheduling decision blocks all subsequent requests** [6]

### Model Loading & Cold Starts

- Models stay loaded for **5 minutes** by default (`OLLAMA_KEEP_ALIVE`) [3]
- After unload, **5-second VRAM recovery wait** before next model load (polls every 250ms until 75% VRAM reclaimed) [6]
- Model option changes (context size, adapters) trigger **full unload + reload** even for running models [6]
- OOM during load triggers retry with reduced context or full eviction — **significant latency spike** [6]

### Context Handling

- Default context auto-scales by VRAM tier: <24 GiB → 4K, 24-48 GiB → 32K, ≥48 GiB → 256K [3]
- `OLLAMA_NUM_PARALLEL × OLLAMA_CONTEXT_LENGTH` determines VRAM consumed by KV cache [3]
- Parallel=4 with 2K context = 8K effective context [3]

### GPU Scheduling

- Prefers single-GPU fit over multi-GPU spread to avoid PCI bus overhead [3]
- Default loaded model cap: **3 × number of GPUs** (`OLLAMA_MAX_LOADED_MODELS`) [3]
- Exceeding this evicts least-recently-active model

### Streaming

- Uses **NDJSON** (newline-delimited JSON) with per-token flush [6]
- Each chunk is a full response object written with `c.Writer.Write(append(data, '\n')); c.Writer.Flush()` [6]
- No binary framing — higher overhead than SSE or gRPC streaming

### Key Difference from vLLM

Ollama uses llama.cpp backend which **does not implement continuous batching** in the same way as vLLM. This is likely the largest throughput difference. vLLM's PagedAttention scheduler can handle many more concurrent requests efficiently, while Ollama's sequential scheduler becomes a bottleneck under load.

---

## 5. Observability Stack Design

### Prometheus Metrics to Collect

**vLLM metrics** (native, no exporter needed) [4][5]:

| Metric | Type | Purpose |
|--------|------|---------|
| `vllm:time_to_first_token_seconds` | Histogram | TTFT distribution |
| `vllm:inter_token_latency_seconds` | Histogram | ITL distribution |
| `vllm:e2e_request_latency_seconds` | Histogram | End-to-end latency |
| `vllm:request_prefill_time_seconds` | Histogram | Prefill phase time |
| `vllm:request_decode_time_seconds` | Histogram | Decode phase time |
| `vllm:request_queue_time_seconds` | Histogram | Queue wait time |
| `vllm:num_requests_running` | Gauge | Current batch occupancy |
| `vllm:num_requests_waiting` | Gauge | Queue depth |
| `vllm:kv_cache_usage_perc` | Gauge | Memory pressure (0-1) |
| `vllm:prefix_cache_hits` / `_queries` | Counter | Cache effectiveness |
| `vllm:num_preemptions` | Counter | Preemption events |
| `vllm:request_time_per_output_token_seconds` | Histogram | Per-token decode cost |

**GPU metrics** (via DCGM exporter):

| Metric | Purpose |
|--------|---------|
| `DCGM_FI_DEV_GPU_UTIL` | GPU utilization % |
| `DCGM_FI_DEV_FB_USED` | GPU memory used |
| `DCGM_FI_DEV_GPU_TEMP` | GPU temperature |
| `DCGM_FI_DEV_POWER_USAGE` | Power consumption |

**Application metrics** (custom):

| Metric | Purpose |
|--------|---------|
| `webhook_received_timestamp` | Webhook ingress timing |
| `kafka_produce_latency_ms` | Kafka enqueue time |
| `kafka_consumer_lag` | Consumer lag |
| `orchestrator_dispatch_duration_ms` | Orchestrator overhead |
| `agent_execution_duration_ms` | Total agent time |
| `agent_tool_call_duration_ms` | Per-tool-call latency |
| `github_api_duration_ms` | GitHub API call time |
| `github_api_rate_limit_remaining` | Rate limit headroom |

**Ollama**: No native Prometheus metrics — requires custom instrumentation or third-party exporters. This is another reason to prioritize the vLLM migration.

### OpenTelemetry Integration

The OpenTelemetry GenAI semantic conventions define standardized metrics and spans [5]:

- **Client metrics**: `gen_ai.client.operation.duration`, `gen_ai.client.operation.time_to_first_chunk`, `gen_ai.client.token.usage`
- **Server metrics**: `gen_ai.server.time_to_first_token`, `gen_ai.server.time_per_output_token`
- **Agent metrics**: `gen_ai.invoke_agent.duration`, `gen_ai.invoke_agent.inference_calls`, `gen_ai.invoke_agent.tool_calls`
- **Workflow metrics**: `gen_ai.workflow.duration`
- **Span naming**: `{gen_ai.operation.name} {gen_ai.request.model}` with CLIENT span kind [5]

### Recommended Grafana Dashboards

**Dashboard 1: LLM Inference Overview**
- TTFT p50/p95/p99 over time
- ITL p50/p95/p99 over time
- Tokens per second (throughput)
- Request rate (QPS)
- Error rate

**Dashboard 2: vLLM Resource Utilization**
- KV cache usage % over time
- Running vs waiting requests
- Preemption rate
- Prefix cache hit rate
- GPU utilization + memory (from DCGM)

**Dashboard 3: End-to-End Pipeline**
- Webhook → Kafka → Orchestrator → Agent → LLM → GitHub API latency waterfall
- Consumer lag over time
- Tool call count and duration per request

**Dashboard 4: Alerting**
- TTFT p95 > 2s (warning), > 5s (critical)
- ITL p95 > 100ms (warning), > 200ms (critical)
- KV cache usage > 90% (warning), > 95% (critical)
- Preemption rate > 0 (any = investigate)
- Consumer lag > 100 (warning), > 1000 (critical)

---

## 6. Debugging Methodology — Step-by-Step Playbook

### Step 1: Check End-to-End Latency
```
# What's the total request duration?
PromQL: histogram_quantile(0.95, rate(vllm:e2e_request_latency_seconds_bucket[5m]))
```
If p95 > 5s, proceed to isolate which stage is slow.

### Step 2: Check vLLM Queue Time
```
# Is the request waiting in queue?
PromQL: histogram_quantile(0.95, rate(vllm:request_queue_time_seconds_bucket[5m]))
```
If queue time > 1s → insufficient capacity. Check `num_requests_waiting` and `kv_cache_usage_perc`.

### Step 3: Check Prefill vs Decode
```
# Is prefill or decode the bottleneck?
PromQL: histogram_quantile(0.95, rate(vllm:request_prefill_time_seconds_bucket[5m]))
PromQL: histogram_quantile(0.95, rate(vllm:request_decode_time_seconds_bucket[5m]))
```
- High prefill, low decode → prompt too long, need prefix caching
- Low prefill, high decode → too many concurrent requests or model too large

### Step 4: Check KV Cache
```
# Is memory the constraint?
PromQL: vllm:kv_cache_usage_perc
```
If > 0.9 → preemption risk. Increase `gpu_memory_utilization` or reduce `max_model_len`.

### Step 5: Check Preemptions
```
# Any preemptions happening?
PromQL: rate(vllm:num_preemptions[5m])
```
Any non-zero value means requests are being evicted and re-prefilled — the most expensive latency event.

### Step 6: Check Prefix Cache
```
# Is prefix caching helping?
PromQL: rate(vllm:prefix_cache_hits[5m]) / rate(vllm:prefix_cache_queries[5m])
```
Low hit rate with shared prefixes → check if prompts differ at token level or block size is too large.

### Step 7: Check GPU
```
# Is GPU saturated?
DCGM_FI_DEV_GPU_UTIL > 90%
DCGM_FI_DEV_FB_USED / DCGM_FI_DEV_FB_TOTAL > 0.95
```
GPU memory pressure → reduce batch size or model size. GPU compute saturation → consider speculative decoding.

### Step 8: Check Orchestrator + Agent
```
# Is the bottleneck in our code?
agent_execution_duration_ms breakdown:
  - prompt_construction_ms
  - tool_call_ms (× count)
  - llm_inference_ms
  - response_assembly_ms
```
If tool_call_ms × count > llm_inference_ms → reduce tool call round-trips.

### Step 9: Check Kafka
```
# Is queue delay significant?
kafka_consumer_lag > 100
kafka_produce_latency_ms > 50
```
Consumer lag → add consumers or check consumer processing speed.

### Step 10: Check GitHub API
```
# Is comment posting delayed?
github_api_rate_limit_remaining < 20
github_api_duration_ms > 500
```
Rate limiting → implement exponential backoff or batch comments.

---

# Part 2 — Agent Planning Research

## 7. Planning in Production Agent Systems

### Comparison Matrix

| System | Planning | Task Decomposition | Execution Loop | Reflection | Checkpointing | Memory | Replanning | Verification |
|--------|----------|-------------------|----------------|------------|---------------|--------|------------|-------------|
| **OpenHands** | Goal-oriented with sub-agents | TaskToolSet delegation | Blocking run loop | Judge LLM audit | Resumable task IDs | Session persistence | Via sub-agents | Goal Completion Loop [9] |
| **Claude Code** | Plan/Act separation | Plan subagent (read-only) | Phase-based workflows | Built into loop | Workflow resume | Context window | Dynamic workflows | Worktree isolation [8] |
| **Codex** | AGENTS.md guidance | Iterative test-until-pass | Isolated sandbox | Test results | Cloud sandbox | AGENTS.md | Re-run in sandbox | Tests must pass [12] |
| **Cline** | Plan/Act mode toggle | Strategy → execution | Human approval per action | Built-in | N/A | N/A | Manual replan | File edits + terminal [13] |
| **Roo Code** | Boomerang Mode | new_task delegation | Mode-based delegation | Via modes | N/A | N/A | Via orchestrator | Mode-specific [14] |
| **Goose** | MCP-based tools | Tool-based | Session persistence | N/A | Session save | Session persistence | N/A | Tool results |

### Key Patterns

**Pattern 1: Plan/Act Separation** (Claude Code, Cline)
- Planning phase is read-only, gathers context without side effects
- Execution phase performs actual work with the plan as guidance
- Clear boundary between "thinking" and "doing"

**Pattern 2: Goal Completion Verification** (OpenHands)
- Separate judge LLM audits whether the objective is provably complete
- Checks for "authoritative evidence" — file contents, command output, test results
- Prevents premature termination

**Pattern 3: Iterative Test Execution** (Codex)
- Each task runs in isolated sandbox
- Continuously runs tests until passing
- AGENTS.md provides project-specific guidance
- Simple but effective verification loop

**Pattern 4: Dynamic Workflows** (Claude Code)
- JavaScript scripts orchestrate 100+ subagents
- Phase-based execution with resume-on-interruption
- Most sophisticated orchestration pattern found

---

## 8. OpenHands Deep Dive

### Architecture

OpenHands (formerly OpenDevin) is an open-source coding agent with a sophisticated planning architecture:

**TaskToolSet**: Parent agent launches sub-agents that handle complex, multi-step tasks autonomously. Each sub-agent runs synchronously — the parent blocks until completion [9].

**Goal Completion Loop**: After each `conversation.run()`, a separate judge LLM audits the transcript for "authoritative evidence" that the objective is provably complete — checking file contents, command output, test results [9].

### Planning Mechanism

- Agent receives a high-level goal
- Decomposes into sub-tasks via TaskToolSet
- Each sub-task runs in its own context
- Judge LLM verifies completion after each run
- If incomplete, agent continues with refined approach

### Memory

- Session-level conversation memory
- Task-level context isolation (each sub-agent has its own)
- No persistent cross-session memory by default

### Sandbox Interaction

- Docker-based sandbox for code execution
- Commands run with full access to filesystem
- Agent can read, write, and execute freely
- Output captured and returned to agent context

### Verification

- **Goal Completion Loop**: Separate judge LLM verifies task completion
- Checks for "authoritative evidence" — not just "agent thinks it's done"
- Strongest verification model found in any agent system

---

## 9. Mastra Capabilities Assessment

### What Mastra Supports

Based on available documentation and framework analysis:

| Capability | Support Level | Notes |
|-----------|---------------|-------|
| **Agent Loop** | ✅ Native | Mastra agents run in a loop with tool calling |
| **Workflows** | ✅ Native | Step-based workflows with branching |
| **Tool Calling** | ✅ Native | Rich tool system for file ops, API calls, etc. |
| **Memory** | ✅ Native | Thread-based memory with persistence options |
| **Streaming** | ✅ Native | SSE streaming for real-time responses |

### What Mastra Lacks

| Capability | Support Level | Recommendation |
|-----------|---------------|----------------|
| **Planning** | ❌ No native support | Build custom planning prompt + parser |
| **Task Graphs / DAGs** | ❌ No native support | Implement as structured plan format |
| **Reflection** | ⚠️ Partial | Can be added via prompt engineering |
| **Replanning** | ❌ No native support | Add as a step in the agent loop |
| **Checkpointing** | ⚠️ Thread-based | Not step-level; need custom implementation |
| **Goal Verification** | ❌ No native support | Add verification step post-execution |

### Decision: Extend Mastra vs. Build Custom

**Recommendation: Build a custom planning layer on top of Mastra.**

Rationale:
1. Mastra's agent loop and tool system are solid foundations
2. Planning is orthogonal to Mastra's core value (agent execution)
3. Custom planning allows PR-review-specific optimizations
4. No need to fork or modify Mastra itself
5. Planning logic can be extracted as a reusable library

---

# Part 3 — Architecture Proposal

## 10. Planning Architecture for Inno-Agent

### High-Level Flow

```
Receive PR Review Task
        ↓
    ┌─────────┐
    │ Planner  │ ← System prompt + PR context + codebase context
    └────┬────┘
         ↓
  Execution Plan (DAG)
  ┌─────────────────────────────────────┐
  │ Step 1: Read changed files          │
  │ Step 2: Build dependency graph      │
  │ Step 3: Search related code         │
  │ Step 4: Read relevant implementations│
  │ Step 5: Build understanding         │
  │ Step 6: Run verification            │
  │ Step 7: Execute review              │
  │ Step 8: Self-check                  │
  │ Step 9: Return result               │
  └─────────────────────────────────────┘
         ↓
    ┌──────────┐
    │ Executor  │ ← Mastra agent loop
    └────┬─────┘
         ↓
    Execute Step N
         ↓
    ┌───────────┐
    │ Reflection │ ← Did this step produce useful output?
    └────┬──────┘
         ↓
    Need Replan?
    ┌────┴────┐
    │  Yes    │ → Back to Planner (with new context)
    │  No     │ → Continue to next step
    └─────────┘
         ↓
    All Steps Complete
         ↓
    ┌──────────┐
    │ Verifier  │ ← Judge LLM: is the review complete and accurate?
    └────┬─────┘
         ↓
    Return Review Comment
```

### Component Design

#### Planner

**Input**: PR metadata (files changed, diff, PR description), system prompt, codebase context
**Output**: Structured execution plan (JSON)

```json
{
  "plan_id": "review-pr-1234",
  "objective": "Provide a thorough code review for PR #1234",
  "steps": [
    {
      "id": "S1",
      "action": "read_files",
      "params": {"files": ["src/auth.ts", "src/auth.test.ts"]},
      "depends_on": [],
      "expected_output": "File contents with line numbers"
    },
    {
      "id": "S2",
      "action": "search_related",
      "params": {"query": "auth middleware usage", "exclude_files": []},
      "depends_on": ["S1"],
      "expected_output": "Related code references"
    },
    {
      "id": "S3",
      "action": "analyze_dependencies",
      "params": {},
      "depends_on": ["S1", "S2"],
      "expected_output": "Dependency graph of changes"
    },
    {
      "id": "S4",
      "action": "execute_review",
      "params": {"focus_areas": ["correctness", "security", "performance"]},
      "depends_on": ["S3"],
      "expected_output": "Review findings"
    },
    {
      "id": "S5",
      "action": "self_check",
      "params": {},
      "depends_on": ["S4"],
      "expected_output": "Validated review"
    }
  ]
}
```

#### Executor

- Receives plan from Planner
- Executes steps in dependency order (DAG traversal)
- Each step maps to a Mastra tool call or agent action
- Reports results to Reflection module after each step

#### Reflection Module

After each step execution:
1. Evaluate: Did the step produce useful output?
2. Compare against `expected_output` from plan
3. If insufficient → trigger replan with additional context
4. If sufficient → proceed to next step

#### Replanner

- Receives: original plan + execution history + reflection feedback
- Outputs: revised plan (may add, remove, or reorder steps)
- Bounded: max 3 replans per review to prevent loops

#### Verifier

- Separate judge LLM (can be smaller/faster model)
- Evaluates: Is the review complete? Are findings accurate? Is the comment well-structured?
- Pattern from OpenHands Goal Completion Loop [9]
- If verification fails → one more replan attempt, then return best-effort result

### Step Types for PR Review

| Step Type | Description | Tool Required |
|-----------|-------------|---------------|
| `read_files` | Read specific file contents | File read tool |
| `search_related` | Search codebase for related code | Grep/search tool |
| `analyze_dependencies` | Build dependency graph from changes | AST parser |
| `read_implementations` | Read full implementations of related code | File read tool |
| `run_verification` | Run tests, linters, type checks | Shell execution |
| `execute_review` | Generate review findings | LLM inference |
| `self_check` | Validate review quality | LLM inference (judge) |
| `return_result` | Format and return final comment | Response assembly |

---

## 11. MVP Design

### Scope

The smallest implementation that delivers value:

1. **Planning prompt** that generates a structured plan from PR context
2. **Plan parser** that converts LLM output to JSON execution plan
3. **Step executor** that follows the plan, executing each step via Mastra tools
4. **Reflection check** after each step (simple heuristic: did we get output?)
5. **No replanning** in MVP — just execute the plan as-is
6. **No separate verifier** — the final review is the output

### Implementation

```
┌──────────────────────────────────────────────┐
│ MVP: Planned PR Review                        │
│                                               │
│ 1. Receive PR → Extract diff + metadata       │
│ 2. Planning LLM call → JSON plan              │
│ 3. Parse plan → Step queue                    │
│ 4. For each step:                             │
│    a. Execute via Mastra tool                 │
│    b. Capture output                          │
│    c. Pass output to next step as context     │
│ 5. Final step: Generate review comment        │
│ 6. Post to GitHub                             │
└──────────────────────────────────────────────┘
```

### Estimated Effort

| Component | Complexity | Effort | Impact |
|-----------|-----------|--------|--------|
| Planning prompt | Low | 2-3 days | High — structured execution |
| Plan parser | Low | 1-2 days | Medium — enables step-by-step |
| Step executor | Medium | 3-5 days | High — core execution loop |
| Reflection (basic) | Low | 1 day | Medium — prevents wasted steps |
| Integration testing | Medium | 2-3 days | High — confidence in system |

**Total MVP**: ~2 weeks

---

## 12. Production Design

### Long-Term Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Production Planning System           │
│                                                       │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐       │
│  │ Planner   │───→│ Executor  │───→│ Verifier  │       │
│  │ (LLM)    │    │ (Mastra) │    │ (Judge)  │       │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘       │
│       │               │               │              │
│       ↓               ↓               ↓              │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐       │
│  │ Plan DB  │    │ Step     │    │ Result   │       │
│  │ (SQLite) │    │ Results  │    │ Store    │       │
│  └──────────┘    └──────────┘    └──────────┘       │
│       │               │               │              │
│       ↓               ↓               ↓              │
│  ┌──────────────────────────────────────────┐       │
│  │           Reflection & Replanning         │       │
│  │  - Per-step quality checks               │       │
│  │  - Automatic replan on failure           │       │
│  │  - Bounded replan count (max 3)          │       │
│  └──────────────────────────────────────────┘       │
│       │                                              │
│       ↓                                              │
│  ┌──────────────────────────────────────────┐       │
│  │         Observability Layer               │       │
│  │  - OTel traces per step                   │       │
│  │  - Prometheus metrics per plan            │       │
│  │  - Grafana dashboards                     │       │
│  └──────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────┘
```

### Production Features

1. **Persistent Plan Storage** — SQLite database storing execution plans, step results, and outcomes for debugging and learning
2. **Step-Level Checkpointing** — Each step result persisted; resume from last successful step on failure
3. **Automatic Replanning** — Reflection module triggers replan when step output is insufficient
4. **Plan Templates** — Pre-built plan templates for common review types (small PR, large refactor, security audit)
5. **Learning from Outcomes** — Track which plan patterns produce better reviews; feed back into planner prompts
6. **Full Observability** — OTel traces for each step, Prometheus metrics for plan execution, Grafana dashboards

---

## 13. Implementation Tasks

### Phase 1: Latency Diagnostics (Week 1-2)

| Task | Complexity | Effort | Impact |
|------|-----------|--------|--------|
| Add vLLM Prometheus metrics to Grafana | Low | 1 day | High — immediate visibility |
| Create TTFT/ITL/KV cache dashboard | Medium | 2 days | High — identify bottlenecks |
| Add DCGM GPU exporter | Low | 1 day | Medium — GPU visibility |
| Instrument orchestrator + agent with custom metrics | Medium | 2-3 days | High — end-to-end visibility |
| Set up alerting rules (TTFT, ITL, KV cache, preemptions) | Low | 1 day | High — proactive detection |
| Add OTel traces across pipeline (Kafka → Orchestrator → Agent → LLM) | High | 3-5 days | High — distributed tracing |
| Run latency baseline measurements | Low | 1 day | Medium — establish benchmarks |

### Phase 2: Latency Optimization (Week 2-4)

| Task | Complexity | Effort | Impact |
|------|-----------|--------|--------|
| Migrate from Ollama to vLLM (if not done) | High | 3-5 days | High — fundamental improvement |
| Enable prefix caching in vLLM | Low | 0.5 days | High — reduces TTFT for shared prompts |
| Reduce tool-calling round-trips (batch file reads) | Medium | 2-3 days | High — reduces 5x latency multiplier |
| Tune vLLM `gpu_memory_utilization` and `max_model_len` | Low | 1 day | Medium — reduce preemptions |
| Implement Kafka consumer lag monitoring | Low | 1 day | Medium — queue visibility |
| Add request-level metrics (`--enable-per-request-metrics`) | Low | 0.5 days | High — per-request debugging |

### Phase 3: Planning MVP (Week 3-5)

| Task | Complexity | Effort | Impact |
|------|-----------|--------|--------|
| Design planning prompt for PR review | Medium | 2-3 days | High — structured execution |
| Implement plan parser (JSON extraction from LLM output) | Low | 1-2 days | Medium — enables step-by-step |
| Build step executor with Mastra tool integration | Medium | 3-5 days | High — core execution loop |
| Add basic reflection (step output validation) | Low | 1 day | Medium — prevents wasted steps |
| Integration testing with real PRs | Medium | 2-3 days | High — confidence in system |
| A/B test planned vs unplanned reviews | Medium | 2-3 days | High — measure improvement |

### Phase 4: Production Planning (Week 5-8)

| Task | Complexity | Effort | Impact |
|------|-----------|--------|--------|
| Add replanning with bounded retry (max 3) | Medium | 2-3 days | High — handles edge cases |
| Implement plan template system | Medium | 2-3 days | Medium — faster for common patterns |
| Add step-level checkpointing (resume on failure) | Medium | 2-3 days | High — reliability |
| Build judge LLM verifier (separate from review agent) | Medium | 2-3 days | High — quality assurance |
| Add plan outcome tracking (learn from results) | High | 3-5 days | Medium — continuous improvement |
| Full OTel instrumentation for planning pipeline | Medium | 2-3 days | High — observability |
| Production hardening + load testing | Medium | 2-3 days | High — reliability |

---

## Open Questions

1. **What model size and GPU hardware are you running?** This determines whether prefill or decode is the dominant latency contributor.
2. **How many tool-calling round-trips does a typical review take?** Each round-trip multiplies total LLM latency [2].
3. **Is the vLLM migration complete?** If still on Ollama, the migration itself is the highest-impact optimization.
4. **What's the current consumer lag in Kafka?** Queue delay could be a significant hidden latency source.
5. **Are you using prefix caching?** For PR reviews with shared system prompts, this can dramatically reduce TTFT [4].
6. **What review quality metrics exist?** Without quality measurement, planning improvements can't be validated.

---

## Sources

[1] GitHub Webhooks Best Practices — https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks (accessed 2026-07-25)

[2] Ollama Tool Calling Documentation — https://docs.ollama.com/capabilities/tool-calling.md (accessed 2026-07-25)

[3] Ollama FAQ — https://docs.ollama.com/faq (accessed 2026-07-25)

[4] vLLM Design: Metrics — https://docs.vllm.ai/en/latest/design/metrics/ (accessed 2026-07-25)

[5] OpenTelemetry GenAI Semantic Conventions — https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-metrics.md (accessed 2026-07-25)

[6] Ollama Source Code (sched.go, routes.go) — https://raw.githubusercontent.com/ollama/ollama/refs/heads/main/server/sched.go (accessed 2026-07-25)

[7] vLLM Architecture Overview — https://docs.vllm.ai/en/latest/design/arch_overview/ (accessed 2026-07-25)

[8] Claude Code Sub-agents Documentation — https://code.claude.com/docs/en/sub-agents (accessed 2026-07-25)

[9] OpenHands Goal Completion Loop — https://docs.openhands.dev/sdk/guides/convo-goal.md (accessed 2026-07-25)

[10] Mastra Documentation — https://mastra.ai/ (accessed 2026-07-25)

[11] Synthesized from findings F1-F6 + architectural analysis

[12] OpenAI Codex Announcement — https://openai.com/index/introducing-codex/ (accessed 2026-07-25)

[13] Cline GitHub Repository — https://github.com/cline/cline (accessed 2026-07-25)

[14] Roo Code Documentation — https://roocodeinc.github.io/Roo-Code/basic-usage/using-modes (accessed 2026-07-25)

[15] vLLM PagedAttention Paper — Kwon et al., SOSP 2023, arXiv:2309.06180

[16] vLLM Speculative Decoding — https://docs.vllm.ai/en/latest/features/speculative_decoding/ (accessed 2026-07-25)

[17] vLLM Automatic Prefix Caching — https://docs.vllm.ai/en/latest/features/automatic_prefix_caching/ (accessed 2026-07-25)

[18] vLLM Per-Request Metrics — https://docs.vllm.ai/en/latest/features/per_request_metrics/ (accessed 2026-07-25)

[19] GitHub Rate Limits — https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api (accessed 2026-07-25)

[20] Ollama Context Length — https://docs.ollama.com/context-length (accessed 2026-07-25)

[21] vLLM KV Cache Metrics Source — https://github.com/vllm-project/vllm/blob/main/vllm/v1/metrics/loggers.py (accessed 2026-07-25)

[22] vLLM Prometheus Integration — https://github.com/vllm-project/vllm/blob/main/vllm/v1/metrics/prometheus.py (accessed 2026-07-25)

[23] vLLM Request Stats — https://github.com/vllm-project/vllm/blob/main/vllm/v1/metrics/stats.py (accessed 2026-07-25)

[24] OpenHands TaskToolSet — https://docs.openhands.dev/sdk/guides/task-tool-set.md (accessed 2026-07-25)

[25] Claude Code Workflows — https://code.claude.com/docs/en/workflows (accessed 2026-07-25)

[26] Ollama Streaming API — https://docs.ollama.com/api/streaming.md (accessed 2026-07-25)

[27] GitHub Webhook Events — https://docs.github.com/en/webhooks/webhook-events-and-payloads (accessed 2026-07-25)

[28] vLLM V1 Architecture — https://docs.vllm.ai/en/latest/design/arch_overview/ (accessed 2026-07-25)

[29] OpenTelemetry GenAI Spans — https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md (accessed 2026-07-25)

[30] NVIDIA DCGM Exporter — https://github.com/NVIDIA/dcgm-exporter (accessed 2026-07-25)
