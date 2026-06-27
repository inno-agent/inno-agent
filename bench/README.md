# Load Testing Infrastructure

Benchmarking framework for the inno-agent project. Measures LLM inference performance across the full stack.

## Architecture

```
Frontend → chat-api:8000 → orchestrator:8080 → Ollama:11434 → LLM
```

The bench tool targets **orchestrator:8080** directly (no auth required) to measure LLM inference performance.

## Quick Start

```bash
# Build
cd bench
go build -o bench ./cmd/bench/

# Run smoke test
./bench --scenario smoke --target http://localhost:8080

# Run concurrency test with 10 users
./bench --scenario concurrency --users 10 --target http://localhost:8080

# Run from config file
./bench --config configs/concurrency.yaml
```

## Scenarios

### 1. Smoke Test
Verifies the system is operational.
```bash
./bench --scenario smoke
```

### 2. Cold Start Test
Measures first-request latency (model loading).
```bash
./bench --scenario coldstart
```

### 3. Warm Start Test
Measures performance after model is loaded (10 sequential requests).
```bash
./bench --scenario warmstart
```

### 4. Concurrency Test
Ramps through user levels: 1, 2, 5, 10, 20, 30, 40, 50, 75, 100.
```bash
./bench --scenario concurrency --users 100
```

### 5. Spike Test
Instantly increases load: 1 → 50 → 100 RPS.
```bash
./bench --scenario spike
```

### 6. Sustained Load Test
Maintains constant load at 10/20/30/40/50 RPS for 10 minutes each.
```bash
./bench --scenario sustained --duration 10m
```

### 7. Queue Test
Gradually increases load to find saturation point.
```bash
./bench --scenario queue
```

### 8. Streaming Test
Tests SSE streaming with multiple concurrent users.
```bash
./bench --scenario streaming --users 5
```

### 9. Recovery Test
Spikes load then verifies system recovery.
```bash
./bench --scenario recovery
```

### 10. Long Prompt Test
Tests with prompts of 500, 2000, 4000, 8000 tokens.
```bash
./bench --scenario longprompt
```

## CLI Options

| Flag | Description | Default |
|------|-------------|---------|
| `--target` | Target URL | `http://localhost:8080` |
| `--scenario` | Scenario name | `smoke` |
| `--users` | Concurrent users | `1` |
| `--rps` | Target RPS | `1` |
| `--duration` | Test duration | `30s` |
| `--timeout` | Request timeout | `180s` |
| `--output` | Results directory | `results` |
| `--message` | Chat message | Hello... |
| `--config` | YAML config file | - |

## Configuration Files

Each scenario has a YAML config in `configs/`:

```yaml
target: "http://localhost:8080"
scenario: "concurrency"
users: 10
duration: 30s
timeout: 180s
message: "Hello, respond with a short greeting."
conc_levels:
  - 1
  - 5
  - 10
  - 20
  - 50
  - 100
```

## Metrics Collected

| Metric | Description |
|--------|-------------|
| Latency | Total request time |
| TTFT | Time to first token (streaming) |
| P50/P95/P99 | Latency percentiles |
| RPS | Requests per second |
| Success Rate | Percentage of successful requests |
| Bytes | Total response bytes |
| Chunks | Total streaming chunks |
| Errors | Failed request count |

## Output

### JSON Results
Saved to `results/YYYY-MM-DD-<scenario>.json`:
```json
{
  "name": "concurrency",
  "started_at": "2026-06-27T10:00:00Z",
  "summary": {
    "total_requests": 100,
    "avg_latency": 1500000000,
    "p50_ms": 1200.5,
    "p95_ms": 2500.3,
    "p99_ms": 3000.1,
    "rps": 3.3,
    "success_rate": 98.0
  },
  "levels": [...]
}
```

### Markdown Report
Generated at `reports/report.md` with tables for each scenario.

### Docker Stats
Automatically collected after each test run.

## Baselines

### Cold Start (Baseline #1)
- Model: qwen2.5:0.5b
- load_duration: ~1.32s
- eval_duration: ~0.52s
- Subsequent requests should have lower load_duration due to KEEP_ALIVE

## Running All Scenarios

```bash
./scripts/run-all.sh http://localhost:8080
```

## Adding New Scenarios

1. Create `internal/scenarios/myscenario.go`:
```go
package scenarios

func RunMyScenario(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
    // Implementation
}
```

2. Add to `cmd/bench/main.go` switch statement

3. Create `configs/myscenario.yaml`

4. Add to `scripts/run-all.sh`

## Project Structure

```
bench/
├── cmd/bench/main.go         # CLI entry point
├── internal/
│   ├── config/config.go      # Configuration
│   ├── client/
│   │   ├── http.go           # HTTP client
│   │   └── sse.go            # SSE streaming client
│   ├── metrics/collector.go  # Metrics aggregation
│   ├── scenarios/            # 10 scenario implementations
│   ├── monitor/docker.go     # Docker stats
│   └── report/               # JSON + Markdown output
├── configs/                  # YAML configs per scenario
├── load/prompts.yaml         # Test prompts
├── results/                  # JSON output (gitignored)
├── reports/                  # Markdown reports
└── scripts/run-all.sh        # Run all scenarios
```
