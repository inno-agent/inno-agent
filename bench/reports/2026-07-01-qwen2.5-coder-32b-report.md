# Load Test Report: qwen2.5-coder:32b

**Date:** 2026-07-01 11:32
**Server:** https://gpu-0.devops-playground.innopolis.university
**GPU:** NVIDIA A100 80GB

## Phase A: Direct Ollama API

### Cold vs Warm

| Metric | Cold | Warm |
|--------|------|------|
| Load time | 7.66s | 0.47s |
| Total latency | 14.01s | 6.78s |
| Tokens/sec | 41.1 | 41.1 |
| TTFT | 7.78s | 0.54s |
| VRAM | — | 28.5 GB |

### Concurrency Scaling

| Concurrency | Success | Avg Latency | P95 Latency | Tokens/sec | Error Rate |
|-------------|---------|-------------|-------------|------------|------------|
| 1 | 1/1 | 3.9s | 3.9s | 41 | 0.0% |
| 2 | 2/2 | 5.5s | 7.1s | 41 | 0.0% |
| 5 | 5/5 | 10.3s | 16.7s | 41 | 0.0% |
| 10 | 10/10 | 18.2s | 32.5s | 41 | 0.0% |
| 20 | 20/20 | 34.3s | 64.8s | 41 | 0.0% |
| 30 | 30/30 | 50.8s | 94.3s | 40 | 0.0% |
| 50 | 50/50 | 83.5s | 156.2s | 40 | 0.0% |

### Sustained Load (33s)

| Metric | Value |
|--------|-------|
| Target RPS | 50 |
| Actual RPS | 0.2 |
| Avg Latency | 4.07s |
| P95 Latency | 4.20s |
| Completed | 8 |
| Errors | 0 |

## Phase B: Through Stack (Orchestrator)

| Metric | Value |
|--------|-------|
| Wall time | 0.00s |
| Answer length | 0 chars |

### Stack Concurrency

| Concurrency | Success | Avg Latency | P95 Latency | Error Rate |
|-------------|---------|-------------|-------------|------------|
| 1 | 0/1 | 0.0s | 0.0s | 100.0% |

## Phase C: Breaking Point

| Concurrency | Success | Errors | Error Rate | VRAM |
|-------------|---------|--------|------------|------|
| 50 | 50/50 | 0 | 0.0% | 28.5 GB |
| 75 | 75/75 | 0 | 0.0% | 28.5 GB |
| 100 | 100/100 | 0 | 0.0% | 28.5 GB |
| 150 | 150/150 | 0 | 0.0% | 28.5 GB |
| 200 | 184/200 | 16 | 8.0% | 28.5 GB |

## Executive Summary

**Model:** qwen2.5-coder:32b

- **Warm tokens/sec:** 41.1
- **Max stable concurrency:** 50
- **VRAM usage:** 28.5 GB / 80 GB