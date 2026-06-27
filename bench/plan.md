# Load Testing Plan

## Roadmap

### Phase 1: Infrastructure ✅
- [x] Create bench/ directory structure
- [x] Go module setup
- [x] HTTP client (non-streaming)
- [x] SSE streaming client
- [x] Metrics collector (percentiles, RPS, success rate)
- [x] Docker stats monitor
- [x] JSON result writer
- [x] Markdown report generator
- [x] CLI with flag parsing
- [x] YAML config support

### Phase 2: Scenarios ✅
- [x] Smoke Test
- [x] Cold Start Test
- [x] Warm Start Test
- [x] Concurrency Test (1→100 users)
- [x] Spike Test (1→50→100 RPS)
- [x] Sustained Load Test (10-50 RPS, 10 min each)
- [x] Queue Saturation Test
- [x] Streaming/SSE Test
- [x] Recovery Test
- [x] Long Prompt Test (500-8000 tokens)

### Phase 3: Documentation ✅
- [x] README.md with usage instructions
- [x] plan.md with roadmap
- [x] Per-scenario YAML configs
- [x] Test prompts at various token sizes
- [x] run-all.sh script

### Phase 4: Validation
- [ ] Build verification
- [ ] Smoke test against running orchestrator
- [ ] Cold start baseline measurement
- [ ] Concurrency test at 10 users
- [ ] Streaming test verification
- [ ] Full test suite execution

## Test Execution Checklist

### Pre-requisites
- [ ] Docker Compose stack is running
- [ ] Orchestrator is accessible at target URL
- [ ] Go 1.26+ is installed

### Smoke Test
- [ ] Health endpoints respond
- [ ] Basic chat request succeeds
- [ ] No errors in logs

### Cold Start Test
- [ ] First request measures load_duration
- [ ] Baseline recorded: ~1.32s load, ~0.52s eval
- [ ] Results saved to results/

### Warm Start Test
- [ ] 10 sequential requests complete
- [ ] Latency improvement measured
- [ ] Comparison with cold start

### Concurrency Test
- [ ] All 10 levels tested (1,2,5,10,20,30,40,50,75,100)
- [ ] P95/P99 calculated per level
- [ ] Error rate tracked
- [ ] Degradation point identified

### Spike Test
- [ ] 1 RPS baseline established
- [ ] Spike to 50 RPS handled
- [ ] Spike to 100 RPS handled
- [ ] Queue behavior observed

### Sustained Load Test
- [ ] 10 RPS for 10 minutes
- [ ] 20 RPS for 10 minutes
- [ ] 30 RPS for 10 minutes
- [ ] 40 RPS for 10 minutes
- [ ] 50 RPS for 10 minutes
- [ ] Memory leak detection
- [ ] Performance degradation tracking

### Queue Test
- [ ] Saturation point identified
- [ ] 503 error threshold found
- [ ] Latency degradation curve mapped

### Streaming Test
- [ ] SSE connections stable
- [ ] TTFT measured
- [ ] Chunk delivery consistent
- [ ] No connection drops

### Recovery Test
- [ ] System recovers after spike
- [ ] Latency returns to baseline
- [ ] No lingering errors

### Long Prompt Test
- [ ] 500 tokens processed
- [ ] 2000 tokens processed
- [ ] 4000 tokens processed
- [ ] 8000 tokens processed
- [ ] Context size impact measured

## Metrics to Track

### Per-Request
- Latency (total request time)
- TTFT (time to first token)
- Bytes received
- Chunks received (streaming)
- Success/failure

### Aggregate
- P50, P95, P99 latency
- RPS (actual vs target)
- Success rate
- Error rate by type (timeout, 5xx, connection)

### System
- CPU usage per container
- Memory usage per container
- Network I/O
- Docker container count

## Baseline Values

### Cold Start (qwen2.5:0.5b)
| Metric | Value |
|--------|-------|
| load_duration | ~1.32s |
| eval_duration | ~0.52s |
| First request latency | ~2.0s |

### Warm Start (qwen2.5:0.5b)
| Metric | Value |
|--------|-------|
| load_duration | <0.1s (cached) |
| eval_duration | ~0.5s |
| Request latency | ~0.6-1.0s |

## Notes

- All tests target orchestrator:8080 directly (no auth)
- Streaming tests use SSE protocol
- Docker stats collected automatically after each run
- Results saved as JSON + Markdown
- Baselines should be updated after infrastructure changes
