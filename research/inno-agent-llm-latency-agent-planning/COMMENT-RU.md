# Исследование: Латентность LLM и Архитектура Планирования Агента

> Inno-Agent · 2026-07-25 · 30+ источников

---

Латентность модели выросла. Гипотеза — узкое место не в нашем коде. Провели полное исследование пайплайна от вебхука GitHub до комментария ревью, а также изучили архитектуры планирования в 9+ системах агентов.

---

## Расследование латентности LLM

Полный пайплайн выглядит так: GitHub PR → Webhook [100-500ms] → Kafka [1-100ms] → Orchestrator [1-5ms] → Agent [50ms-30s] → LLM Inference [500ms-60s] → GitHub API [50-200ms].

Webhook — GitHub ждёт ответ за 10 секунд, иначе считает доставку провалившейся. Полезная метрика: `webhook_received_timestamp` vs `webhook_processed_timestamp`.

Kafka — типичная латентность 1-100ms, но при rebalancing consumer group — паузы в секунды. Метрики: `kafka_consumer_lag`, `kafka_produce_latency_ms`.

Orchestrator — Go-based, пренебрежимо мал (1-5ms), если не заблокирован downstream.

Agent — каждый tool call = полный LLM round-trip (prefill + decode). Ревью с 5 tool calls = 5x латентность LLM. Это ключевой фактор.

LLM Inference — доминирующая стадия. Включает очередь, prefill и decode.

GitHub API — secondary rate limits: 80 content-creating запросов в минуту. При burst-нагрузке постинг комментария может задержаться.

---

### Что может быть узким местом

GPU: насыщение утилизации (>90%) ведёт к высокому ITL. Давление на память (>95%) — OOM kill'ы и премпшены. Термический троттлинг — постепенное падение производительности. Мульти-GPU PCIe transfer — латентность выше чем на одном GPU.

CPU и память: Go GC pause даёт периодические скачки латентности в оркестраторе. Node.js event loop blocking — Mastra agent зависает. Docker container memory limits — OOM kill'ы и рестарты.

Сеть и очереди: Kafka consumer lag — задержка обработки ревью. Docker bridge networking — добавленная латентность между контейнерами. Reverse proxy — +1-5ms на каждый хоп.

Модель: длинный prompt prefill — высокий TTFT. KV cache exhaustion — премпшены, очереди растут. Cold start (загрузка модели) — секунды задержки.

---

### vLLM: Что нужно знать

Prefill обрабатывает весь входной промпт через все слои трансформера. Доминирует в TTFT. С линейной зависимостью от длины промпта. Метрики: `vllm:request_prefill_time_seconds`, `vllm:time_to_first_token_seconds`.

Decode генерирует токены по одному. Определяет ITL. Метрики: `vllm:inter_token_latency_seconds`, `vllm:request_time_per_output_token_seconds`.

Continuous Batching — новые запросы присоединяются к активному batch между шагами decode. Нет head-of-line blocking. Метрики: `vllm:num_requests_running`, `vllm:num_requests_waiting`.

Paged Attention — KV cache хранится фиксированными блоками (страницами) на GPU. Устраняет фрагментацию памяти. Метрики: `vllm:kv_cache_usage_perc`, `vllm:kv_block_lifetime_seconds`.

Prefix Caching кэширует KV блоки, чтобы новые запросы с общим префиксом пропускали пересчёт. Только помогает prefill, не decode. Включается через `--enable-prefix-caching`.

Speculative Decoding — draft model предлагает кандидаты, target model валидирует параллельно. Lossless. Эффективен при low-to-medium QPS.

Chunked Prefill разбивает длинный prefill на несколько forward passes, чередуя с decode. Улучшает стабильность ITL.

Премпшены — самое дорогое событие по латентности — полный re-prefill с нуля. Метрика: `vllm:num_preemptions`. Любое ненулевое значение = скачки латентности.

---

### Ollama: Проблемы

Очередь 512 запросов перед HTTP 503. Сериализация загрузки моделей — только одна загрузка за раз. Однопоточный планировщик (processPending) — одно медленное решение блокирует все остальные запросы. 5 минут до выгрузки модели (OLLAMA_KEEP_ALIVE). 5-секундное ожидание восстановления VRAM после выгрузки. Нет нативных Prometheus метрик — невозможно мониторить без кастомной инструментации. Нет continuous batching в стиле vLLM — главная причина разницы в пропускной способности.

Вывод: переход на vLLM — правильное решение для production.

---

### Observability: Что добавить

VLLM метрики (нативные, без экспортера): `vllm:time_to_first_token_seconds` (TTFT), `vllm:inter_token_latency_seconds` (ITL), `vllm:e2e_request_latency_seconds` (end-to-end), `vllm:request_prefill_time_seconds` (prefill), `vllm:request_decode_time_seconds` (decode), `vllm:request_queue_time_seconds` (очередь), `vllm:num_requests_running` / `_waiting` (загрузка), `vllm:kv_cache_usage_perc` (давление на память), `vllm:prefix_cache_hits` / `_queries` (эффективность кэша), `vllm:num_preemptions` (премпшены).

GPU метрики (DCGM exporter): `DCGM_FI_DEV_GPU_UTIL` (утилизация), `DCGM_FI_DEV_FB_USED` (память), `DCGM_FI_DEV_GPU_TEMP` (температура).

Кастомные метрики приложения: `orchestrator_dispatch_duration_ms`, `agent_execution_duration_ms`, `agent_tool_call_duration_ms`, `kafka_consumer_lag`, `github_api_duration_ms`.

Grafana дашборды (рекомендуемые): LLM Inference Overview — TTFT/ITL p50/p95/p99, throughput, QPS, errors. vLLM Resource Utilization — KV cache, running/waiting requests, preemptions, prefix cache hit rate, GPU. End-to-End Pipeline — waterfall latency через все стадии. Alerting — TTFT p95 > 2s, ITL p95 > 100ms, KV cache > 90%, preemptions > 0, consumer lag > 100.

---

### Методология отладки — 10 шагов

1. Проверить end-to-end латентность: `histogram_quantile(0.95, rate(vllm:e2e_request_latency_seconds_bucket[5m]))`. Если p95 > 5s — изолировать стадию.

2. Проверить очередь vLLM: `histogram_quantile(0.95, rate(vllm:request_queue_time_seconds_bucket[5m]))`. Если > 1s — недостаточно capacity.

3. Prefill vs Decode: сравнить `request_prefill_time_seconds` и `request_decode_time_seconds`. High prefill + low decode — промпт длинный, нужен prefix caching.

4. KV Cache: `vllm:kv_cache_usage_perc`. Если > 0.9 — риск премпшенов.

5. Премпшены: `rate(vllm:num_preemptions[5m])`. Любое ненулевое значение — запросы пересчитываются с нуля.

6. Prefix Cache: `rate(vllm:prefix_cache_hits[5m]) / rate(vllm:prefix_cache_queries[5m])`. Низкий hit rate — промпты различаются на уровне токенов.

7. GPU: `DCGM_FI_DEV_GPU_UTIL > 90%`, `DCGM_FI_DEV_FB_USED / TOTAL > 0.95`. Давление на память — уменьшить batch или модель.

8. Orchestrator + Agent: разбивка `agent_execution_duration_ms` — prompt_construction, tool_call × count, llm_inference, response_assembly. Если tool_call × count > llm_inference — уменьшить количество round-trips.

9. Kafka: `kafka_consumer_lag > 100`, `kafka_produce_latency_ms > 50`.

10. GitHub API: `github_api_rate_limit_remaining < 20`, `github_api_duration_ms > 500`.

---

## Исследование планирования агентов

### Как планируют в production системах

OpenHands использует Goal-oriented подход с sub-agents через TaskToolSet. Родительский агент блокируется до завершения каждого sub-agent. После каждого conversation.run() отдельный judge LLM проверяет транскрипт на "authoritative evidence" завершённости — содержимое файлов, вывод команд, результаты тестов. Это strongest verification model из всех исследованных систем.

Claude Code использует Plan/Act separation — отдельный Plan subagent (read-only) собирает контекст, затем execution запускается в основном контексте. Dynamic Workflows оркестрируют 100+ subagents через JS-скрипты с phase-based execution и resume-on-interruption.

Codex (OpenAI) запускает каждую задачу в изолированном cloud sandbox, непрерывно запуская тесты до прохождения. AGENTS.md даёт project-specific guidance.

Cline использует Plan/Act toggle — Plan mode исследует и строит стратегию, Act mode выполняет с human approval на каждое действие. Kanban enables parallel multi-agent execution с per-card worktrees.

Roo Code использует Boomerang Mode — стратегический оркестратор, который декомпозирует сложные задачи и делегирует их специализированным режимам через new_task tool.

Ключевые паттерны: (1) Plan/Act Separation — фаза планирования read-only, фаза выполнения по плану, чёткая граница между "думать" и "делать". (2) Goal Completion Verification — отдельный judge LLM проверяет доказательную базу завершённости. (3) Iterative Test Execution — непрерывный запуск тестов до прохождения. (4) Dynamic Workflows — масштабная оркестрация через скрипты.

---

### OpenHands: Deep Dive

TaskToolSet — родительский агент запускает sub-agents для сложных задач. Каждый sub-agent работает синхронно — родитель блокируется до завершения.

Goal Completion Loop — после каждого conversation.run() отдельный judge LLM проверяет транскрипт на "authoritative evidence" завершённости. Проверяет содержимое файлов, вывод команд, результаты тестов.

Планирование: агент получает высокую цель, декомпозирует через TaskToolSet, каждый sub-task в своём контексте, judge LLM верифицирует завершение, если неполно — агент продолжает с уточнённым подходом.

Верификация — strongest verification model. Отдельный LLM проверяет не просто "агент считает задачу выполненной", а ищет доказательства.

---

### Mastra: Оценка возможностей

Mastra поддерживает: Agent Loop (нативно — агенты работают в цикле с tool calling), Workflows (нативно — step-based workflows с branching), Tool Calling (нативно — богатая система инструментов), Memory (нативно — thread-based memory с persistence), Streaming (нативно — SSE streaming).

Mastra НЕ поддерживает: Planning (нет нативной поддержки), Task Graphs / DAGs (нет нативной поддержки), Reflection (частично, через prompt engineering), Replanning (нет нативной поддержки), Checkpointing (thread-based, не step-level), Goal Verification (нет нативной поддержки).

Решение: строить кастомный planning layer поверх Mastra. Причины: agent loop и tool system Mastra — хорошая основа. Планирование ортогонально ядру Mastra. Кастомное планирование позволяет оптимизации под PR review. Не нужно форкать или модифицировать Mastra. Planning logic можно вынести как переиспользуемую библиотеку.

---

## Предложение архитектуры

Архитектура планирования для Inno-Agent: Получение PR Review Task → Planner (System prompt + PR context + codebase context) → Execution Plan (DAG) → Executor (Mastra agent loop) → Execute Step N → Reflection (Шаг дал полезный вывод?) → Нужен Replan? → Да: Back to Planner с новым контекстом. Нет: Следующий шаг. Все шаги выполнены → Verifier (Judge LLM: ревью полное и точное?) → Return Review Comment.

Компоненты: Planner — LLM генерирует JSON-план (steps с depends_on, params, expected_output). Executor — выполняет шаги по DAG через Mastra tools. Reflection — после каждого шага проверяет качество вывода. Replanner — получает план + историю + feedback, генерирует修订 план (макс. 3 replan'а). Verifier — отдельный judge LLM проверяет итоговое ревью.

Типы шагов для PR review: read_files (чтение файлов, File read tool), search_related (поиск связанного кода, Grep/search), analyze_dependencies (граф зависимостей, AST parser), read_implementations (чтение реализаций, File read), run_verification (запуск тестов/lint, Shell), execute_review (генерация ревью, LLM inference), self_check (проверка качества, LLM judge), return_result (форматирование вывода, Response assembly).

---

### MVP

Самая маленькая реализация, дающая ценность: (1) Planning prompt — генерирует структурированный план из PR контекста. (2) Plan parser — конвертирует LLM output в JSON execution plan. (3) Step executor — выполняет план через Mastra tools. (4) Reflection check — простая эвристика после каждого шага (есть вывод?). (5) Без replanning — выполняем план как есть. (6) Без отдельного verifier — финальный review = вывод.

Сложность и трудозатраты: Planning prompt — низкая, 2-3 дня, высокое влияние. Plan parser — низкая, 1-2 дня, среднее влияние. Step executor — средняя, 3-5 дней, высокое влияние. Reflection (базовый) — низкая, 1 день, среднее влияние. Integration testing — средняя, 2-3 дня, высокое влияние. Итого MVP: ~2 недели.

---

### Production Design

Долгосрочная архитектура: (1) Persistent Plan Storage — SQLite для хранения планов, результатов шагов, исходов (для отладки и обучения). (2) Step-Level Checkpointing — каждый результат шага персистится; resume с последнего успешного шага при сбое. (3) Automatic Replanning — Reflection триггерит replan когда вывод шага недостаточен. (4) Plan Templates — готовые шаблоны для типичных типов ревью (маленький PR, большой refactor, security audit). (5) Learning from Outcomes — трекинг какие паттерны планов дают лучшие ревью; обратная связь в planner prompts. (6) Full Observability — OTel traces для каждого шага, Prometheus метрики для execution, Grafana дашборды.

---

## Задачи по реализации

### Фаза 1: Диагностика латентности (Неделя 1-2)

Добавить vLLM Prometheus метрики в Grafana (низкая, 1 день, высокое). Создать TTFT/ITL/KV cache дашборд (средняя, 2 дня, высокое). Добавить DCGM GPU exporter (низкая, 1 день, среднее). Инструментировать orchestrator + agent кастомными метриками (средняя, 2-3 дня, высокое). Настроить alerting — TTFT, ITL, KV cache, preemptions (низкая, 1 день, высокое). Добавить OTel traces через пайплайн (высокая, 3-5 дней, высокое). Запустить baseline замеры латентности (низкая, 1 день, среднее).

### Фаза 2: Оптимизация латентности (Неделя 2-4)

Миграция Ollama → vLLM если не завершена (высокая, 3-5 дней, высокое). Включить prefix caching в vLLM (низкая, 0.5 дня, высокое). Уменьшить tool-calling round-trips — batch file reads (средняя, 2-3 дня, высокое). Тюнинг gpu_memory_utilization и max_model_len (низкая, 1 день, среднее). Мониторинг Kafka consumer lag (низкая, 1 день, среднее). Добавить request-level metrics —enable-per-request-metrics (низкая, 0.5 дня, высокое).

### Фаза 3: Planning MVP (Неделя 3-5)

Спроектировать planning prompt для PR review (средняя, 2-3 дня, высокое). Реализовать plan parser — JSON из LLM output (низкая, 1-2 дня, среднее). Построить step executor с Mastra tool integration (средняя, 3-5 дней, высокое). Добавить базовый reflection (низкая, 1 день, среднее). Интеграционное тестирование с реальными PR (средняя, 2-3 дня, высокое). A/B тест: planned vs unplanned reviews (средняя, 2-3 дня, высокое).

### Фаза 4: Production Planning (Неделя 5-8)

Добавить replanning с bounded retry — макс. 3 (средняя, 2-3 дня, высокое). Реализовать систему plan templates (средняя, 2-3 дня, среднее). Добавить step-level checkpointing — resume при сбое (средняя, 2-3 дня, высокое). Построить judge LLM verifier (средняя, 2-3 дня, высокое). Добавить plan outcome tracking — обучение по результатам (высокая, 3-5 дней, среднее). Полная OTel инструментация planning pipeline (средняя, 2-3 дня, высокое). Production hardening + load testing (средняя, 2-3 дня, высокое).

---

## Открытые вопросы

Какая модель и GPU? Это определяет, prefill или decode — доминирующий вкладчик в латентность. Сколько tool-calling round-trips в типичном ревью? Каждый round-trip умножает общую латентность LLM. Завершена ли миграция на vLLM? Если всё ещё на Ollama — сама миграция даёт наибольший эффект. Какой текущий consumer lag в Kafka? Задержка очереди может быть значительным скрытым источником. Используете ли prefix caching? Для PR review с общими system prompts это может радикально снизить TTFT. Есть ли метрики качества ревью? Без измерения качества нельзя валидировать улучшения от планирования.

---

## Источники

[1] GitHub Webhooks Best Practices — docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks

[2] Ollama Tool Calling — docs.ollama.com/capabilities/tool-calling.md

[3] Ollama FAQ — docs.ollama.com/faq

[4] vLLM Design: Metrics — docs.vllm.ai/en/latest/design/metrics/

[5] OpenTelemetry GenAI Semantic Conventions — github.com/open-telemetry/semantic-conventions-genai

[6] Ollama Source Code (sched.go, routes.go) — github.com/ollama/ollama

[7] vLLM Architecture Overview — docs.vllm.ai/en/latest/design/arch_overview/

[8] Claude Code Sub-agents — code.claude.com/docs/en/sub-agents

[9] OpenHands Goal Completion Loop — docs.openhands.dev/sdk/guides/convo-goal.md

[10] OpenAI Codex — openai.com/index/introducing-codex/

[11] Cline GitHub — github.com/cline/cline

[12] Roo Code Docs — roocodeinc.github.io/Roo-Code

[13] vLLM PagedAttention Paper — Kwon et al., SOSP 2023, arXiv:2309.06180

[14] vLLM Speculative Decoding — docs.vllm.ai/en/latest/features/speculative_decoding/

[15] vLLM Automatic Prefix Caching — docs.vllm.ai/en/latest/features/automatic_prefix_caching/

[16] NVIDIA DCGM Exporter — github.com/NVIDIA/dcgm-exporter

[17] GitHub Rate Limits — docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api

[18] Ollama Context Length — docs.ollama.com/context-length

[19] Claude Code Workflows — code.claude.com/docs/en/workflows

[20] OpenHands TaskToolSet — docs.openhands.dev/sdk/guides/task-tool-set.md
