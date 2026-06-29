#!/usr/bin/env python3
"""Remote Ollama A100 benchmark script."""
import json
import time
import sys
import statistics
import concurrent.futures
import urllib.request
import urllib.error

BASE = "https://gpu-0.devops-playground.innopolis.university"
AUTH = "Bearer dhigimalXa0IqBCB6uxmGX1WydZ09Voi1ufB+bUJaVc="
PROMPT = "Write a Python function that checks if a number is prime. Include docstring and type hints."
WARM_PROMPT = PROMPT

def api(method, path, data=None):
    url = BASE + path
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Authorization", AUTH)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=300) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return {"error": f"HTTP {e.code}", "body": e.read().decode()[:200]}

def get_tags():
    return api("GET", "/api/tags")

def unload_model(model):
    """Unload a model by setting keep_alive to 0."""
    api("POST", "/api/generate", {"model": model, "keep_alive": 0, "prompt": ""})

def benchmark_single(model, prompt, run_label=""):
    """Run a single inference request and measure all timings."""
    data = {
        "model": model,
        "prompt": prompt,
        "stream": False,
        "options": {"num_predict": 256}
    }
    
    t_start = time.perf_counter()
    resp = api("POST", "/api/generate", data)
    t_end = time.perf_counter()
    
    wall_time = t_end - t_start
    
    if "error" in resp:
        return {"error": resp["error"], "wall_time": wall_time}
    
    result = {
        "wall_time": wall_time,
        "load_duration_s": resp.get("load_duration", 0) / 1e9,
        "prompt_eval_duration_s": resp.get("prompt_eval_duration", 0) / 1e9,
        "eval_duration_s": resp.get("eval_duration", 0) / 1e9,
        "total_duration_s": resp.get("total_duration", 0) / 1e9,
        "prompt_eval_count": resp.get("prompt_eval_count", 0),
        "eval_count": resp.get("eval_count", 0),
        "response_length": len(resp.get("response", "")),
        "response_preview": resp.get("response", "")[:120],
    }
    
    if result["eval_count"] > 0 and result["eval_duration_s"] > 0:
        result["tokens_per_sec"] = result["eval_count"] / result["eval_duration_s"]
    else:
        result["tokens_per_sec"] = 0
    
    if result["prompt_eval_count"] > 0 and result["prompt_eval_duration_s"] > 0:
        result["prompt_tokens_per_sec"] = result["prompt_eval_count"] / result["prompt_eval_duration_s"]
    else:
        result["prompt_tokens_per_sec"] = 0
    
    # TTFT approximation: load + prompt_eval for non-streaming
    result["ttft_approx_s"] = result["load_duration_s"] + result["prompt_eval_duration_s"]
    
    return result

def parallel_request(model, prompt, request_id):
    """Single parallel request for concurrency testing."""
    t_start = time.perf_counter()
    result = benchmark_single(model, prompt, f"parallel-{request_id}")
    t_end = time.perf_counter()
    result["wall_time"] = t_end - t_start
    result["request_id"] = request_id
    return result

def benchmark_concurrency(model, concurrency, prompt):
    """Run N concurrent requests and collect stats."""
    results = []
    t_start = time.perf_counter()
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = {
            executor.submit(parallel_request, model, prompt, i): i
            for i in range(concurrency)
        }
        for future in concurrent.futures.as_completed(futures):
            try:
                results.append(future.result())
            except Exception as e:
                results.append({"error": str(e), "request_id": futures[future]})
    
    t_end = time.perf_counter()
    
    successes = [r for r in results if "error" not in r]
    errors = [r for r in results if "error" in r]
    
    stats = {
        "concurrency": concurrency,
        "total_wall_time": t_end - t_start,
        "successes": len(successes),
        "errors": len(errors),
        "error_messages": [r.get("error", "") for r in errors],
    }
    
    if successes:
        wall_times = [r["wall_time"] for r in successes]
        tokens = [r.get("tokens_per_sec", 0) for r in successes if r.get("tokens_per_sec", 0) > 0]
        ttfts = [r.get("ttft_approx_s", 0) for r in successes]
        
        stats["avg_wall_time"] = statistics.mean(wall_times)
        stats["p95_wall_time"] = sorted(wall_times)[int(len(wall_times) * 0.95)] if len(wall_times) >= 2 else wall_times[0]
        stats["max_wall_time"] = max(wall_times)
        stats["min_wall_time"] = min(wall_times)
        stats["avg_ttft"] = statistics.mean(ttfts)
        stats["avg_tokens_per_sec"] = statistics.mean(tokens) if tokens else 0
        stats["throughput"] = sum(r.get("eval_count", 0) for r in successes) / (t_end - t_start)
    
    return stats

def main():
    models_to_test = [
        "qwen2.5:0.5b",
        "qwen2.5-coder:1.5b",
        "llama3.2:1b",
        "qwen2.5:7b",
        "qwen2.5-coder:7b",
        "qwen3:8b",
        "qwen2.5-coder:14b",
        "deepseek-coder-v2:16b",
        "codestral:latest",
        "qwen3:32b",
    ]
    
    all_results = {}
    
    print("=" * 80)
    print("REMOTE OLLAMA A100 BENCHMARK")
    print("=" * 80)
    
    # Phase 1: Single request benchmarks (cold + warm)
    print("\n--- PHASE 1: Cold Start & Warm Benchmarks ---\n")
    
    for model in models_to_test:
        print(f"\n>>> Testing {model}...")
        
        # Cold start: unload first
        print(f"  Unloading {model}...")
        unload_model(model)
        time.sleep(2)
        
        # Cold request
        print(f"  Cold request...")
        cold = benchmark_single(model, PROMPT, "cold")
        
        # Warm request (model is now loaded)
        print(f"  Warm request...")
        warm = benchmark_single(model, WARM_PROMPT, "warm")
        
        all_results[model] = {"cold": cold, "warm": warm}
        
        if "error" in cold:
            print(f"  ERROR: {cold['error']}")
            continue
        
        print(f"  Cold: load={cold['load_duration_s']:.2f}s  prompt_eval={cold['prompt_eval_duration_s']:.2f}s  "
              f"eval={cold['eval_duration_s']:.2f}s  total={cold['total_duration_s']:.2f}s  "
              f"tokens={cold['eval_count']}  tps={cold['tokens_per_sec']:.1f}")
        print(f"  Warm: load={warm['load_duration_s']:.2f}s  prompt_eval={warm['prompt_eval_duration_s']:.2f}s  "
              f"eval={warm['eval_duration_s']:.2f}s  total={warm['total_duration_s']:.2f}s  "
              f"tokens={warm['eval_count']}  tps={warm['tokens_per_sec']:.1f}")
        
        if cold['load_duration_s'] > 0:
            savings = (1 - warm['total_duration_s'] / cold['total_duration_s']) * 100 if cold['total_duration_s'] > 0 else 0
            print(f"  Warm savings: {savings:.0f}% latency reduction")
    
    # Phase 2: Concurrency benchmarks
    print("\n\n--- PHASE 2: Concurrency Benchmarks ---\n")
    
    concurrency_levels = [1, 5, 10, 20, 30, 50]
    
    # Test concurrency on key models
    concurrency_models = [
        "qwen2.5:0.5b",
        "qwen2.5-coder:1.5b",
        "qwen2.5:7b",
        "qwen2.5-coder:7b",
        "qwen3:8b",
        "qwen2.5-coder:14b",
        "deepseek-coder-v2:16b",
        "codestral:latest",
    ]
    
    concurrency_results = {}
    
    for model in concurrency_models:
        print(f"\n>>> Concurrency test: {model}")
        concurrency_results[model] = []
        
        # Ensure model is loaded
        benchmark_single(model, "warmup", "warmup")
        
        for level in concurrency_levels:
            print(f"  Concurrency={level}...", end=" ", flush=True)
            stats = benchmark_concurrency(model, level, PROMPT)
            concurrency_results[model].append(stats)
            
            err_str = f" errors={stats['errors']}" if stats['errors'] > 0 else ""
            avg_tps = stats.get('avg_tokens_per_sec', 0)
            print(f"wall={stats.get('avg_wall_time', 0):.1f}s  "
                  f"p95={stats.get('p95_wall_time', 0):.1f}s  "
                  f"tps={avg_tps:.1f}{err_str}")
            
            # If high error rate, stop increasing
            if stats['errors'] > level * 0.5:
                print(f"  >> Too many errors, stopping concurrency test for {model}")
                break
        
        # Unload after test
        unload_model(model)
        time.sleep(1)
    
    # Save results
    output = {
        "single": all_results,
        "concurrency": concurrency_results,
    }
    
    with open("/home/stoat/code/inno-agent/bench_results.json", "w") as f:
        json.dump(output, f, indent=2, default=str)
    
    print("\n\nResults saved to bench_results.json")

if __name__ == "__main__":
    main()
