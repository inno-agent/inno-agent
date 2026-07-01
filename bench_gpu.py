#!/usr/bin/env python3
"""
bench_gpu.py — qwen2.5-coder:32b load testing for A100 server.

Tests:
  Phase A: Direct Ollama API (clean model metrics)
  Phase B: Through our stack (orchestrator + chat-api)
  Phase C: Breaking point analysis

Usage:
  python3 bench_gpu.py                    # Run all phases
  python3 bench_gpu.py --phase a          # Direct Ollama only
  python3 bench_gpu.py --phase b          # Through stack only
  python3 bench_gpu.py --skip-pull        # Skip model pull
  python3 bench_gpu.py --model qwen3:32b  # Test different model
"""
import json
import time
import sys
import statistics
import concurrent.futures
import urllib.request
import urllib.error
import argparse
import os
from datetime import datetime

# ── Config ───────────────────────────────────────────────────────────────────
OLLAMA_BASE = "https://gpu-0.devops-playground.innopolis.university"
OLLAMA_KEY = "dhigimalXa0IqBCB6uxmGX1WydZ09Voi1ufB+bUJaVc="
STACK_BASE = "http://localhost:8080"  # orchestrator
CHAT_BASE = "http://localhost:8000"   # chat-api

DEFAULT_MODEL = "qwen2.5-coder:32b"
PROMPT = "Write a Python function that implements binary search on a sorted array. Include type hints, docstring with examples, and error handling for empty array."
SHORT_PROMPT = "Say hello in one word."

CONCURRENCY_LEVELS = [1, 2, 5, 10, 20, 30, 50]
SUSTAINED_DURATION = 30  # seconds
WARMUP_REQUESTS = 2

# ── HTTP helpers ─────────────────────────────────────────────────────────────
def ollama_api(method, path, data=None, timeout=300):
    """Call Ollama API with auth."""
    url = f"{OLLAMA_BASE}{path}"
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Authorization", f"Bearer {OLLAMA_KEY}")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return {"error": f"HTTP {e.code}", "body": e.read().decode()[:200]}
    except Exception as e:
        return {"error": str(e)}

def stack_api(method, path, data=None, timeout=300):
    """Call our stack API (no auth for local testing)."""
    url = f"{STACK_BASE}{path}"
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return {"error": f"HTTP {e.code}", "body": e.read().decode()[:200]}
    except Exception as e:
        return {"error": str(e)}

def chat_api(method, path, data=None, timeout=300):
    """Call chat-api (with auth token if available)."""
    url = f"{CHAT_BASE}{path}"
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Content-Type", "application/json")
    token = os.environ.get("AUTH_TOKEN", "")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return {"error": f"HTTP {e.code}", "body": e.read().decode()[:200]}
    except Exception as e:
        return {"error": str(e)}

# ── Model management ─────────────────────────────────────────────────────────
def pull_model(model):
    """Pull model to remote server."""
    print(f"\n{'='*60}")
    print(f"PULLING: {model}")
    print(f"{'='*60}")

    # Check if already exists
    tags = ollama_api("GET", "/api/tags")
    if "models" in tags:
        for m in tags["models"]:
            if m["name"] == model or m["name"].startswith(f"{model}:"):
                size_gb = m.get("size", 0) / 1e9
                print(f"  Already installed ({size_gb:.1f} GB)")
                return True

    print(f"  Starting pull...")
    t0 = time.time()

    # Stream pull progress
    url = f"{OLLAMA_BASE}/api/pull"
    body = json.dumps({"name": model}).encode()
    req = urllib.request.Request(url, data=body, method="POST")
    req.add_header("Authorization", f"Bearer {OLLAMA_KEY}")
    req.add_header("Content-Type", "application/json")

    try:
        with urllib.request.urlopen(req, timeout=1800) as resp:
            last_status = ""
            for line in resp:
                try:
                    data = json.loads(line)
                    status = data.get("status", "")
                    if status != last_status:
                        elapsed = time.time() - t0
                        print(f"  [{elapsed:.0f}s] {status}")
                        last_status = status
                except json.JSONDecodeError:
                    pass
    except Exception as e:
        print(f"  Pull failed: {e}")
        return False

    elapsed = time.time() - t0
    print(f"  Pull completed in {elapsed:.1f}s")
    return True

def unload_model(model):
    """Unload model from GPU."""
    ollama_api("POST", "/api/generate", {"model": model, "keep_alive": 0, "prompt": ""})
    time.sleep(2)

def get_gpu_status():
    """Get GPU status from Ollama."""
    ps = ollama_api("GET", "/api/ps")
    if "models" in ps:
        for m in ps["models"]:
            if DEFAULT_MODEL in m.get("name", ""):
                return {
                    "vram_bytes": m.get("size_vram", 0),
                    "vram_gb": m.get("size_vram", 0) / 1e9,
                    "context_length": m.get("context_length", 0),
                }
    return {"vram_bytes": 0, "vram_gb": 0, "context_length": 0}

# ── Single request benchmark ─────────────────────────────────────────────────
def bench_single_direct(model, prompt, max_tokens=256):
    """Benchmark a single request directly to Ollama."""
    data = {
        "model": model,
        "prompt": prompt,
        "stream": False,
        "options": {"num_predict": max_tokens}
    }
    t0 = time.perf_counter()
    resp = ollama_api("POST", "/api/generate", data)
    wall = time.perf_counter() - t0

    if "error" in resp:
        return {"error": resp["error"], "wall_time": wall}

    result = {
        "wall_time": wall,
        "load_duration_s": resp.get("load_duration", 0) / 1e9,
        "prompt_eval_duration_s": resp.get("prompt_eval_duration", 0) / 1e9,
        "eval_duration_s": resp.get("eval_duration", 0) / 1e9,
        "total_duration_s": resp.get("total_duration", 0) / 1e9,
        "prompt_eval_count": resp.get("prompt_eval_count", 0),
        "eval_count": resp.get("eval_count", 0),
        "response_length": len(resp.get("response", "")),
    }

    if result["eval_count"] > 0 and result["eval_duration_s"] > 0:
        result["tokens_per_sec"] = result["eval_count"] / result["eval_duration_s"]
    else:
        result["tokens_per_sec"] = 0

    result["ttft_s"] = result["load_duration_s"] + result["prompt_eval_duration_s"]
    return result

def bench_single_stack(prompt, model="auto", max_tokens=256):
    """Benchmark a single request through our stack."""
    data = {
        "messages": [{"role": "user", "content": prompt}],
        "model_name": model,
        "stream": False,
    }
    t0 = time.perf_counter()
    resp = stack_api("POST", "/v1/chat", data)
    wall = time.perf_counter() - t0

    if "error" in resp:
        return {"error": resp["error"], "wall_time": wall}

    return {
        "wall_time": wall,
        "answer_length": len(resp.get("answer", "")),
    }

# ── Concurrency benchmark ───────────────────────────────────────────────────
def bench_concurrent_direct(model, concurrency, prompt, max_tokens=128):
    """Run N concurrent requests directly to Ollama."""
    results = []
    t0 = time.perf_counter()

    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as ex:
        futures = {
            ex.submit(bench_single_direct, model, prompt, max_tokens): i
            for i in range(concurrency)
        }
        for f in concurrent.futures.as_completed(futures):
            try:
                results.append(f.result())
            except Exception as e:
                results.append({"error": str(e), "wall_time": 0})

    total_wall = time.perf_counter() - t0
    return analyze_results(results, total_wall, concurrency)

def bench_concurrent_stack(concurrency, prompt, model="auto", max_tokens=128):
    """Run N concurrent requests through our stack."""
    results = []
    t0 = time.perf_counter()

    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as ex:
        futures = {
            ex.submit(bench_single_stack, prompt, model, max_tokens): i
            for i in range(concurrency)
        }
        for f in concurrent.futures.as_completed(futures):
            try:
                results.append(f.result())
            except Exception as e:
                results.append({"error": str(e), "wall_time": 0})

    total_wall = time.perf_counter() - t0
    return analyze_results(results, total_wall, concurrency)

def analyze_results(results, total_wall, concurrency):
    """Analyze concurrent request results."""
    ok = [r for r in results if "error" not in r]
    err = [r for r in results if "error" in r]

    stats = {
        "concurrency": concurrency,
        "total_wall": total_wall,
        "successes": len(ok),
        "errors": len(err),
        "error_rate": len(err) / concurrency * 100 if concurrency > 0 else 0,
    }

    if ok:
        walls = [r["wall_time"] for r in ok]
        stats["avg_wall"] = statistics.mean(walls)
        stats["p50_wall"] = sorted(walls)[len(walls)//2]
        stats["p95_wall"] = sorted(walls)[int(len(walls)*0.95)] if len(walls) >= 2 else walls[0]
        stats["max_wall"] = max(walls)

        tps = [r.get("tokens_per_sec", 0) for r in ok if r.get("tokens_per_sec", 0) > 0]
        if tps:
            stats["avg_tps"] = statistics.mean(tps)
            stats["total_tokens"] = sum(r.get("eval_count", 0) for r in ok)
            stats["throughput"] = stats["total_tokens"] / total_wall if total_wall > 0 else 0

    if err:
        error_types = {}
        for e in err:
            msg = e.get("error", "unknown")[:50]
            error_types[msg] = error_types.get(msg, 0) + 1
        stats["error_types"] = error_types

    return stats

# ── Sustained load ───────────────────────────────────────────────────────────
def bench_sustained_direct(model, target_rps, duration, prompt, max_tokens=128):
    """Send requests at target_rps for duration seconds."""
    interval = 1.0 / target_rps if target_rps > 0 else 1.0
    results = []
    errors = 0
    t_start = time.perf_counter()

    while time.perf_counter() - t_start < duration:
        t_req = time.perf_counter()
        r = bench_single_direct(model, prompt, max_tokens)
        elapsed = time.perf_counter() - t_req

        if "error" in r:
            errors += 1
        else:
            results.append(r)

        next_time = t_req + interval
        sleep_time = next_time - time.perf_counter()
        if sleep_time > 0:
            time.sleep(sleep_time)

    actual_duration = time.perf_counter() - t_start
    total = len(results) + errors

    stats = {
        "target_rps": target_rps,
        "actual_rps": total / actual_duration,
        "completed": len(results),
        "errors": errors,
        "duration": actual_duration,
    }

    if results:
        walls = [r["wall_time"] for r in results]
        stats["avg_latency"] = statistics.mean(walls)
        stats["p95_latency"] = sorted(walls)[int(len(walls)*0.95)]

    return stats

# ── Phase A: Direct Ollama ──────────────────────────────────────────────────
def phase_a_direct(model):
    """Phase A: Direct Ollama API benchmarks."""
    print(f"\n{'#'*60}")
    print(f"# PHASE A: Direct Ollama API — {model}")
    print(f"{'#'*60}")

    results = {"model": model, "phase": "direct", "tests": {}}

    # 1. Pull model
    if not pull_model(model):
        print("FATAL: Could not pull model")
        return None

    # 2. Cold start
    print(f"\n--- Cold Start Test ---")
    unload_model(model)
    time.sleep(3)
    cold = bench_single_direct(model, PROMPT, 256)
    results["tests"]["cold_start"] = cold
    print(f"  Cold: load={cold.get('load_duration_s',0):.2f}s  "
          f"total={cold.get('total_duration_s',0):.2f}s  "
          f"tps={cold.get('tokens_per_sec',0):.1f}")

    # 3. Warm baseline
    print(f"\n--- Warm Baseline ---")
    for i in range(WARMUP_REQUESTS):
        bench_single_direct(model, SHORT_PROMPT, 32)
    warm = bench_single_direct(model, PROMPT, 256)
    results["tests"]["warm_baseline"] = warm
    gpu = get_gpu_status()
    results["tests"]["gpu_baseline"] = gpu
    print(f"  Warm: total={warm.get('total_duration_s',0):.2f}s  "
          f"tps={warm.get('tokens_per_sec',0):.1f}  "
          f"vram={gpu.get('vram_gb',0):.1f} GB")

    # 4. Concurrency sweep
    print(f"\n--- Concurrency Sweep ---")
    results["tests"]["concurrency"] = []
    for c in CONCURRENCY_LEVELS:
        print(f"  c={c}...", end=" ", flush=True)
        # Warm up before each level
        bench_single_direct(model, SHORT_PROMPT, 32)
        stats = bench_concurrent_direct(model, c, PROMPT, 128)
        results["tests"]["concurrency"].append(stats)
        gpu = get_gpu_status()
        stats["gpu"] = gpu
        err = f" errors={stats['errors']}" if stats.get("errors", 0) > 0 else ""
        print(f"ok={stats['successes']} wall={stats.get('avg_wall',0):.1f}s "
              f"p95={stats.get('p95_wall',0):.1f}s "
              f"tps={stats.get('avg_tps',0):.0f}{err}")
        if stats.get("error_rate", 0) > 50:
            print(f"  >> High error rate, stopping concurrency sweep")
            break

    # 5. Sustained load
    print(f"\n--- Sustained Load (30s) ---")
    # Find optimal concurrency from sweep
    optimal_c = 5
    for s in results["tests"]["concurrency"]:
        if s.get("error_rate", 100) < 10:
            optimal_c = s["concurrency"]
    print(f"  Using concurrency={optimal_c}")
    sustained = bench_sustained_direct(model, optimal_c, SUSTAINED_DURATION, PROMPT, 128)
    results["tests"]["sustained"] = sustained
    print(f"  RPS={sustained.get('actual_rps',0):.1f}  "
          f"avg={sustained.get('avg_latency',0):.2f}s  "
          f"p95={sustained.get('p95_latency',0):.2f}s")

    return results

# ── Phase B: Through Stack ──────────────────────────────────────────────────
def phase_b_stack(model):
    """Phase B: Through our stack (orchestrator + chat-api)."""
    print(f"\n{'#'*60}")
    print(f"# PHASE B: Through Stack — {model}")
    print(f"{'#'*60}")

    results = {"model": model, "phase": "stack", "tests": {}}

    # Check if stack is running
    health = stack_api("GET", "/health")
    if "error" in health:
        print(f"  WARNING: Orchestrator not reachable: {health['error']}")
        print(f"  Starting stack test anyway (may fail)...")

    # 1. Warm baseline
    print(f"\n--- Stack Warm Baseline ---")
    for i in range(WARMUP_REQUESTS):
        bench_single_stack(SHORT_PROMPT, model, 32)
    warm = bench_single_stack(PROMPT, model, 256)
    results["tests"]["warm_baseline"] = warm
    print(f"  Stack: wall={warm.get('wall_time',0):.2f}s  "
          f"answer_len={warm.get('answer_length',0)}")

    # 2. Concurrency sweep
    print(f"\n--- Stack Concurrency Sweep ---")
    results["tests"]["concurrency"] = []
    stack_levels = [1, 2, 5, 10, 20]
    for c in stack_levels:
        print(f"  c={c}...", end=" ", flush=True)
        bench_single_stack(SHORT_PROMPT, model, 32)
        stats = bench_concurrent_stack(c, PROMPT, model, 128)
        results["tests"]["concurrency"].append(stats)
        err = f" errors={stats['errors']}" if stats.get("errors", 0) > 0 else ""
        print(f"ok={stats['successes']} wall={stats.get('avg_wall',0):.1f}s "
              f"p95={stats.get('p95_wall',0):.1f}s{err}")
        if stats.get("error_rate", 100) > 50:
            print(f"  >> High error rate, stopping")
            break

    return results

# ── Phase C: Breaking Point ─────────────────────────────────────────────────
def phase_c_breaking(model):
    """Phase C: Find the breaking point."""
    print(f"\n{'#'*60}")
    print(f"# PHASE C: Breaking Point — {model}")
    print(f"{'#'*60}")

    results = {"model": model, "phase": "breaking", "tests": {}}

    # Push until we hit errors
    print(f"\n--- Finding Breaking Point ---")
    results["tests"]["breaking"] = []
    for c in [50, 75, 100, 150, 200]:
        print(f"  c={c}...", end=" ", flush=True)
        bench_single_direct(model, SHORT_PROMPT, 32)
        stats = bench_concurrent_direct(model, c, PROMPT, 64)
        results["tests"]["breaking"].append(stats)
        gpu = get_gpu_status()
        stats["gpu"] = gpu
        err = f" errors={stats['errors']}" if stats.get("errors", 0) > 0 else ""
        print(f"ok={stats['successes']} wall={stats.get('avg_wall',0):.1f}s "
              f"vram={gpu.get('vram_gb',0):.1f}GB{err}")
        if stats.get("error_rate", 0) > 80:
            print(f"  >> Breaking point reached at c={c}")
            break

    return results

# ── Report generation ────────────────────────────────────────────────────────
def generate_report(all_results, model):
    """Generate markdown report."""
    ts = datetime.now().strftime("%Y-%m-%d %H:%M")
    lines = [
        f"# Load Test Report: {model}",
        f"",
        f"**Date:** {ts}",
        f"**Server:** {OLLAMA_BASE}",
        f"**GPU:** NVIDIA A100 80GB",
        f"",
    ]

    for phase_data in all_results:
        phase = phase_data.get("phase", "unknown")
        tests = phase_data.get("tests", {})

        if phase == "direct":
            lines.append(f"## Phase A: Direct Ollama API")
            lines.append(f"")

            # Cold vs Warm
            cold = tests.get("cold_start", {})
            warm = tests.get("warm_baseline", {})
            gpu = tests.get("gpu_baseline", {})

            lines.append(f"### Cold vs Warm")
            lines.append(f"")
            lines.append(f"| Metric | Cold | Warm |")
            lines.append(f"|--------|------|------|")
            lines.append(f"| Load time | {cold.get('load_duration_s',0):.2f}s | {warm.get('load_duration_s',0):.2f}s |")
            lines.append(f"| Total latency | {cold.get('total_duration_s',0):.2f}s | {warm.get('total_duration_s',0):.2f}s |")
            lines.append(f"| Tokens/sec | {cold.get('tokens_per_sec',0):.1f} | {warm.get('tokens_per_sec',0):.1f} |")
            lines.append(f"| TTFT | {cold.get('ttft_s',0):.2f}s | {warm.get('ttft_s',0):.2f}s |")
            lines.append(f"| VRAM | — | {gpu.get('vram_gb',0):.1f} GB |")
            lines.append(f"")

            # Concurrency
            conc = tests.get("concurrency", [])
            if conc:
                lines.append(f"### Concurrency Scaling")
                lines.append(f"")
                lines.append(f"| Concurrency | Success | Avg Latency | P95 Latency | Tokens/sec | Error Rate |")
                lines.append(f"|-------------|---------|-------------|-------------|------------|------------|")
                for s in conc:
                    lines.append(f"| {s['concurrency']} | {s['successes']}/{s['concurrency']} | "
                                 f"{s.get('avg_wall',0):.1f}s | {s.get('p95_wall',0):.1f}s | "
                                 f"{s.get('avg_tps',0):.0f} | {s.get('error_rate',0):.1f}% |")
                lines.append(f"")

            # Sustained
            sust = tests.get("sustained", {})
            if sust:
                lines.append(f"### Sustained Load ({sust.get('duration',0):.0f}s)")
                lines.append(f"")
                lines.append(f"| Metric | Value |")
                lines.append(f"|--------|-------|")
                lines.append(f"| Target RPS | {sust.get('target_rps',0)} |")
                lines.append(f"| Actual RPS | {sust.get('actual_rps',0):.1f} |")
                lines.append(f"| Avg Latency | {sust.get('avg_latency',0):.2f}s |")
                lines.append(f"| P95 Latency | {sust.get('p95_latency',0):.2f}s |")
                lines.append(f"| Completed | {sust.get('completed',0)} |")
                lines.append(f"| Errors | {sust.get('errors',0)} |")
                lines.append(f"")

        elif phase == "stack":
            lines.append(f"## Phase B: Through Stack (Orchestrator)")
            lines.append(f"")

            warm = tests.get("warm_baseline", {})
            lines.append(f"| Metric | Value |")
            lines.append(f"|--------|-------|")
            lines.append(f"| Wall time | {warm.get('wall_time',0):.2f}s |")
            lines.append(f"| Answer length | {warm.get('answer_length',0)} chars |")
            lines.append(f"")

            conc = tests.get("concurrency", [])
            if conc:
                lines.append(f"### Stack Concurrency")
                lines.append(f"")
                lines.append(f"| Concurrency | Success | Avg Latency | P95 Latency | Error Rate |")
                lines.append(f"|-------------|---------|-------------|-------------|------------|")
                for s in conc:
                    lines.append(f"| {s['concurrency']} | {s['successes']}/{s['concurrency']} | "
                                 f"{s.get('avg_wall',0):.1f}s | {s.get('p95_wall',0):.1f}s | "
                                 f"{s.get('error_rate',0):.1f}% |")
                lines.append(f"")

        elif phase == "breaking":
            lines.append(f"## Phase C: Breaking Point")
            lines.append(f"")

            breaking = tests.get("breaking", [])
            if breaking:
                lines.append(f"| Concurrency | Success | Errors | Error Rate | VRAM |")
                lines.append(f"|-------------|---------|--------|------------|------|")
                for s in breaking:
                    gpu = s.get("gpu", {})
                    lines.append(f"| {s['concurrency']} | {s['successes']}/{s['concurrency']} | "
                                 f"{s['errors']} | {s.get('error_rate',0):.1f}% | "
                                 f"{gpu.get('vram_gb',0):.1f} GB |")
                lines.append(f"")

    # Executive Summary
    lines.append(f"## Executive Summary")
    lines.append(f"")
    lines.append(f"**Model:** {model}")
    lines.append(f"")
    for phase_data in all_results:
        tests = phase_data.get("tests", {})
        warm = tests.get("warm_baseline", {})
        if warm.get("tokens_per_sec"):
            lines.append(f"- **Warm tokens/sec:** {warm['tokens_per_sec']:.1f}")
            break
    for phase_data in all_results:
        tests = phase_data.get("tests", {})
        conc = tests.get("concurrency", [])
        if conc:
            max_ok = max((s for s in conc if s.get("error_rate", 100) < 10),
                        key=lambda s: s["concurrency"], default=None)
            if max_ok:
                lines.append(f"- **Max stable concurrency:** {max_ok['concurrency']}")
                break
    for phase_data in all_results:
        tests = phase_data.get("tests", {})
        gpu = tests.get("gpu_baseline", {})
        if gpu.get("vram_gb", 0) > 0:
            lines.append(f"- **VRAM usage:** {gpu['vram_gb']:.1f} GB / 80 GB")
            break

    return "\n".join(lines)

# ── Main ─────────────────────────────────────────────────────────────────────
def main():
    parser = argparse.ArgumentParser(description="qwen2.5-coder:32b load testing")
    parser.add_argument("--model", default=DEFAULT_MODEL, help="Model to test")
    parser.add_argument("--phase", choices=["a", "b", "c", "all"], default="all",
                       help="Phase to run (a=direct, b=stack, c=breaking, all=everything)")
    parser.add_argument("--skip-pull", action="store_true", help="Skip model pull")
    parser.add_argument("--output", default="bench_gpu_results.json", help="Output JSON file")
    args = parser.parse_args()

    print(f"{'='*60}")
    print(f"GPU Load Test: {args.model}")
    print(f"Server: {OLLAMA_BASE}")
    print(f"Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"{'='*60}")

    all_results = []

    if args.phase in ("a", "all"):
        if not args.skip_pull:
            pull_model(args.model)
        result_a = phase_a_direct(args.model)
        if result_a:
            all_results.append(result_a)

    if args.phase in ("b", "all"):
        result_b = phase_b_stack(args.model)
        if result_b:
            all_results.append(result_b)

    if args.phase in ("c", "all"):
        result_c = phase_c_breaking(args.model)
        if result_c:
            all_results.append(result_c)

    # Save JSON
    with open(args.output, "w") as f:
        json.dump(all_results, f, indent=2, default=str)
    print(f"\nJSON results: {args.output}")

    # Generate report
    report = generate_report(all_results, args.model)
    report_path = "bench_gpu_report.md"
    with open(report_path, "w") as f:
        f.write(report)
    print(f"Report: {report_path}")

    print(f"\n{'='*60}")
    print(f"DONE")
    print(f"{'='*60}")

if __name__ == "__main__":
    main()
