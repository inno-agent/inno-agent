# Исследование: Латентность LLM и Архитектура Планирования Агента

> Inno-Agent · 2026-07-25 · 30+ источников

---

## Краткое резюме

Латентность модели выросла. Гипотеза — узкое место не в нашем коде. Провели полное исследование пайплайна от вебхука GitHub до комментария ревью, а также изучили архитектуры планирования в 9+ системах агентов. Ниже — основные выводы и рекомендации.

---

## Часть 1: Расследование латентности LLM

### 1. Полный пайплайн латентности

```
GitHub PR → Webhook [100-500ms] → Kafka [1-100ms] → Orchestrator [1-5ms]
→ Agent [50ms-30s] → LLM Inference [500ms-60s] → GitHub API [50-200ms]
```

Каждый этап:

- **Webhook** — GitHub ждёт ответ за 10 секунд, иначе считает доставку провалившейся [1]. Полезная метрика: `webhook_received_timestamp` vs `webhook_processed_timestamp`.

- **Kafka** — типичная латентность 1-100ms, но при rebalancing consumer group — паузы в секунды. Метрики: `kafka_consumer_lag`, `kafka_produce_latency_ms`.

- **Orchestrator** — Go-based, пренебрежимо мал (1-5ms), если не заблокирован downstream.

- **Agent** — **каждый tool call = полный LLM round-trip (prefill + decode)** [2]. Ревью с 5 tool calls = **5x латентность LLM**. Это ключевой фактор.

- **LLM Inference** — доминирующая стадия. Включает очередь, prefill и decode. Подробнее ниже.

- **GitHub API** — secondary rate limits: **80 content-creating запросов в минуту** [1]. При burst-нагрузке постинг комментария может задержаться.

### 2. Что может быть узким местом

**GPU:**
- Насыщение утилизации (>90%) → высокий ITL
- Давление на память (>95%) → OOM kill'ы, премпшены
- Термический троттлинг → постепенное падение производительности
- Мульти-GPU PCIe transfer → латентность выше чем на одном GPU

**CPU и память:**
- Go GC pause → периодические скачки латентности в оркестраторе
- Node.js event loop blocking → Mastra agent зависает
- Docker container memory limits → OOM kill'ы, рестарты

**Сеть и очереди:**
- Kafka consumer lag → задержка обработки ревью
- Docker bridge networking → добавленная латентность между контейнерами
- Reverse proxy → +1-5ms на каждый хоп

**Модель:**
- Длинный prompt prefill → высокий TTFT
- KV cache exhaustion → премпшены, очереди растут
- Cold start (загрузка модели) → секунды задержки

### 3. vLLM: Что нужно знать

**Prefill** — обрабатывает весь входной промпт через все слои трансформера. Доминирует в TTFT. С линейной зависимостью от длины промпта. Метрики: `vllm:request_prefill_time_seconds`, `vllm:time_to_first_token_seconds`.

**Decode** — генерирует токены по одному. Определяет ITL. Метрики: `vllm:inter_token_latency_seconds`, `vllm:request_time_per_output_token_seconds`.

**Continuous Batching** — новые запросы присоединяются к активному batch между шагами decode. Нет head-of-line blocking. Метрики: `vllm:num_requests_running`, `vllm:num_requests_waiting`.

**Paged Attention** — KV cache хранится фиксированными блоками (страницами) на GPU. Устраняет фрагментацию памяти. Метрики: `vllm:kv_cache_usage_perc`, `vllm:kv_block_lifetime_seconds`.

**Prefix Caching** — кэширует KV блоки, чтобы новые запросы с общим префиксом пропускали пересчёт. **Только помогает prefill, не decode** [4]. Включается через `--enable-prefix-caching`.

**Speculative Decoding** — draft model предлагает кандидаты, target model валидирует параллельно. Lossless. Эффективен при low-to-medium QPS.

**Chunked Prefill** — разбивает длинный prefill на несколько forward passes, чередуя с decode. Улучшает стабильность ITL.

**Премпшены** — **самое дорогое событие по латентности** — полный re-prefill с нуля. Метрика: `vllm:num_preemptions`. Любое ненулевое значение = скачки латентности.

### 4. Ollama: Проблемы

- **Очередь 512 запросов** перед HTTP 503 [3][6]
- **Сериализация загрузки моделей** — только одна загрузка за раз [6]
- **Однопоточный планировщик** (`processPending`) — одно медленное решение блокирует все остальные запросы [6]
- **5 минут до выгрузки модели** (`OLLAMA_KEEP_ALIVE`) [3]
- **5-секундное ожидание восстановления VRAM** после выгрузки [6]
- **Нет нативных Prometheus метрик** — невозможно мониторить без кастомной инструментации
- **Нет continuous batching** в стиле vLLM — главная причина разницы в пропускной способности

**Вывод: Переход на vLLM — правильное решение для production.**

### 5. Observability: Что добавить

**VLLM метрики** (нативные, без экспортера) [4][5]:

- `vllm:time_to_first_token_seconds` — TTFT
- `vllm:inter_token_latency_seconds` — ITL
- `vllm:e2e_request_latency_seconds` — end-to-end
- `vllm:request_prefill_time_seconds` — prefill
- `vllm:request_decode_time_seconds` — decode
- `vllm:request_queue_time_seconds` — очередь
- `vllm:num_requests_running` / `_waiting` — загрузка
- `vllm:kv_cache_usage_perc` — давление на память
- `vllm:prefix_cache_hits` / `_queries` — эффективность кэша
- `vllm:num_preemptions` — премпшены

**GPU метрики** (DCGM exporter):

- `DCGM_FI_DEV_GPU_UTIL` — утилизация GPU
- `DCGM_FI_DEV_FB_USED` — память GPU
- `DCGM_FI_DEV_GPU_TEMP` — температура

**Кастомные метрики приложения:**

- `orchestrator_dispatch_duration_ms`
- `agent_execution_duration_ms`
- `agent_tool_call_duration_ms`
- `kafka_consumer_lag`
- `github_api_duration_ms`

**Grafana дашборды** (рекомендуемые):

1. **LLM Inference Overview** — TTFT/ITL p50/p95/p99, throughput, QPS, errors
2. **vLLM Resource Utilization** — KV cache, running/waiting requests, preemptions, prefix cache hit rate, GPU
3. **End-to-End Pipeline** — waterfall latency через все стадии
4. **Alerting** — TTFT p95 > 2s, ITL p95 > 100ms, KV cache > 90%, preemptions > 0, consumer lag > 100

### 6. Методология отладки — 10 шагов

1. **Проверить end-to-end латентность** — `histogram_quantile(0.95, rate(vllm:e2e_request_latency_seconds_bucket[5m]))`. Если p95 > 5s → изолировать стадию.

2. **Проверить очередь vLLM** — `histogram_quantile(0.95, rate(vllm:request_queue_time_seconds_bucket[5m]))`. Если > 1s → недостаточно_capacity.

3. **Prefill vs Decode** — сравнить `request_prefill_time_seconds` и `request_decode_time_seconds`. High prefill + low decode → промпт длинный, нужен prefix caching.

4. **KV Cache** — `vllm:kv_cache_usage_perc`. Если > 0.9 → риск премпшенов.

5. **Премпшены** — `rate(vllm:num_preemptions[5m])`. Любое ненулевое значение → запросы пересчитываются с нуля.

6. **Prefix Cache** — `rate(vllm:prefix_cache_hits[5m]) / rate(vllm:prefix_cache_queries[5m])`. Низкий hit rate → промпты различаются на уровне токенов.

7. **GPU** — `DCGM_FI_DEV_GPU_UTIL > 90%`, `DCGM_FI_DEV_FB_USED / TOTAL > 0.95`. Давление на память → уменьшить batch или модель.

8. **Orchestrator + Agent** — разбивка `agent_execution_duration_ms`: prompt_construction, tool_call × count, llm_inference, response_assembly. Если tool_call × count > llm_inference → уменьшить количество round-trips.

9. **Kafka** — `kafka_consumer_lag > 100`, `kafka_produce_latency_ms > 50`.

10. **GitHub API** — `github_api_rate_limit_remaining < 20`, `github_api_duration_ms > 500`.

---

## Часть 2: Исследование планирования агентов

### 7. Как планируют в production системах

**Сравнительная матрица:**

| Система | Планирование | Декомпозиция | Execution Loop | Reflection | Чекпоинты | Память | Replanning | Верификация |
|---------|-------------|-------------|----------------|------------|-----------|--------|------------|-------------|
| OpenHands | Goal-oriented + sub-agents | TaskToolSet | Blocking run | Judge LLM | Resumable task IDs | Session | Via sub-agents | Goal Completion Loop |
| Claude Code | Plan/Act separation | Plan subagent (read-only) | Phase-based workflows | Built-in | Workflow resume | Context window | Dynamic workflows | Worktree isolation |
| Codex | AGENTS.md guidance | Iterative test-until-pass | Isolated sandbox | Test results | Cloud sandbox | AGENTS.md | Re-run in sandbox | Tests must pass |
| Cline | Plan/Act toggle | Strategy → execution | Human approval per action | Built-in | — | — | Manual replan | File edits + terminal |
| Roo Code | Boomerang Mode | new_task delegation | Mode-based | Via modes | — | — | Via orchestrator | Mode-specific |

**Ключевые паттерны:**

1. **Plan/Act Separation** (Claude Code, Cline) — фаза планирования read-only, собирает контекст без side effects. Фаза выполнения работает по плану. Чёткая граница между "думать" и "делать".

2. **Goal Completion Verification** (OpenHands) — отдельный judge LLM проверяет, доказательно ли задача выполнена. Проверяет "authoritative evidence" — содержимое файлов, вывод команд, результаты тестов.

3. **Iterative Test Execution** (Codex) — каждая задача в изолированном sandbox, непрерывно запускает тесты до прохождения. AGENTS.md даёт project-specific guidance.

4. **Dynamic Workflows** (Claude Code) — JS-скрипты оркестрируют 100+ subagents с phase-based execution и resume-on-interruption.

### 8. OpenHands: Deep Dive

**TaskToolSet** — родительский агент запускает sub-agents для сложных задач. Каждый sub-agent работает синхронно — родитель блокируется до завершения [9].

**Goal Completion Loop** — после каждого `conversation.run()` отдельный judge LLM проверяет транскрипт на "authoritative evidence" завершённости — содержимое файлов, вывод команд, результаты тестов [9].

**Планирование:**
- Агент получает высокую цель
- Декомпозирует через TaskToolSet
- Каждый sub-task в своём контексте
- Judge LLM верифицирует завершение
- Если неполно — агент продолжает с уточнённым подходом

**Верификация** — strongest verification model из всех исследованных систем. Отдельный LLM проверяет не просто "агент считает задачу выполненной", а ищет доказательства.

### 9. Mastra: Оценка возможностей

**Что Mastra поддерживает:**
- Agent Loop (нативно) — агенты работают в цикле с tool calling
- Workflows (нативно) — step-based workflows с branching
- Tool Calling (нативно) — богатая система инструментов
- Memory (нативно) — thread-based memory с persistence
- Streaming (нативно) — SSE streaming

**Чего Mastra НЕ поддерживает:**
- **Planning** — нет нативной поддержки
- **Task Graphs / DAGs** — нет нативной поддержки
- **Reflection** — частично, через prompt engineering
- **Replanning** — нет нативной поддержки
- **Checkpointing** — thread-based, не step-level
- **Goal Verification** — нет нативной поддержки

**Решение: Строить кастомный planning layer поверх Mastra.**

Причины:
1. Agent loop и tool system Mastra — хорошая основа
2. Планирование ортогонально ядру Mastra
3. Кастомное планирование позволяет оптимизации под PR review
4. Не нужно форкать или модифицировать Mastra
5. Planning logic можно вынести как переиспользуемую библиотеку

---

## Часть 3: Предложение архитектуры

### 10. Архитектура планирования для Inno-Agent

```
Получение PR Review Task
        ↓
    ┌─────────┐
    │ Planner  │ ← System prompt + PR context + codebase context
    └────┬────┘
         ↓
  Execution Plan (DAG)
  ┌─────────────────────────────────────┐
  │ Step 1: Прочитать изменённые файлы  │
  │ Step 2: Построить граф зависимостей │
  │ Step 3: Найти связанный код         │
  │ Step 4: Прочитать реализации        │
  │ Step 5: Понять контекст             │
  │ Step 6: Запустить верификацию       │
  │ Step 7: Выполнить ревью             │
  │ Step 8: Self-check                  │
  │ Step 9: Вернуть результат           │
  └─────────────────────────────────────┘
         ↓
    ┌──────────┐
    │ Executor  │ ← Mastra agent loop
    └────┬─────┘
         ↓
    Execute Step N
         ↓
    ┌───────────┐
    │ Reflection │ ← Шаг дал полезный вывод?
    └────┬──────┘
         ↓
    Нужен Replan?
    ┌────┴────┐
    │  Да     │ → Back to Planner (с новым контекстом)
    │  Нет    │ → Следующий шаг
    └─────────┘
         ↓
    Все шаги выполнены
         ↓
    ┌──────────┐
    │ Verifier  │ ← Judge LLM: ревью полное и точное?
    └────┬─────┘
         ↓
    Return Review Comment
```

**Компоненты:**

- **Planner** — LLM генерирует JSON-план (steps с depends_on, params, expected_output)
- **Executor** — выполняет шаги по DAG через Mastra tools
- **Reflection** — после каждого шага проверяет качество вывода
- **Replanner** — получает план + историю + feedback, генерирует修订 план (макс. 3 replan'а)
- **Verifier** — отдельный judge LLM проверяет итоговое ревью

**Типы шагов для PR review:**

| Шаг | Описание | Инструмент |
|-----|----------|-----------|
| `read_files` | Чтение файлов | File read tool |
| `search_related` | Поиск связанного кода | Grep/search |
| `analyze_dependencies` | Граф зависимостей | AST parser |
| `read_implementations` | Чтение реализаций | File read |
| `run_verification` | Запуск тестов/lint | Shell |
| `execute_review` | Генерация ревью | LLM inference |
| `self_check` | Проверка качества | LLM (judge) |
| `return_result` | Форматирование вывода | Response assembly |

### 11. MVP

**Самая маленькая реализация, дающая ценность:**

1. **Planning prompt** — генерирует структурированный план из PR контекста
2. **Plan parser** — конвертирует LLM output в JSON execution plan
3. **Step executor** — выполняет план через Mastra tools
4. **Reflection check** — простая эвристика после каждого шага (есть вывод?)
5. **Без replanning** — выполняем план как есть
6. **Без отдельного verifier** — финальный review = вывод

**Сложность и трудозатраты:**

| Компонент | Сложность | Трудозатраты | Влияние |
|-----------|----------|-------------|---------|
| Planning prompt | Низкая | 2-3 дня | Высокое |
| Plan parser | Низкая | 1-2 дня | Среднее |
| Step executor | Средняя | 3-5 дней | Высокое |
| Reflection (базовый) | Низкая | 1 день | Среднее |
| Integration testing | Средняя | 2-3 дня | Высокое |

**Итого MVP: ~2 недели**

### 12. Production Design

**Долгосрочная архитектура:**

1. **Persistent Plan Storage** — SQLite для хранения планов, результатов шагов, исходов (для отладки и обучения)
2. **Step-Level Checkpointing** — каждый результат шага персистится; resume с последнего успешного шага при сбое
3. **Automatic Replanning** — Reflection триггерит replan когда вывод шага недостаточен
4. **Plan Templates** — готовые шаблоны для типичных типов ревью (маленький PR, большой refactor, security audit)
5. **Learning from Outcomes** — трекинг какие паттерны планов дают лучшие ревью; обратная связь в planner prompts
6. **Full Observability** — OTel traces для каждого шага, Prometheus метрики для execution, Grafana дашборды

---

## Часть 4: Задачи по реализации

### Фаза 1: Диагностика латентности (Неделя 1-2)

| Задача | Сложность | Трудозатраты | Влияние |
|--------|----------|-------------|---------|
| Добавить vLLM Prometheus метрики в Grafana | Низкая | 1 день | Высокое |
| Создать TTFT/ITL/KV cache дашборд | Средняя | 2 дня | Высокое |
| Добавить DCGM GPU exporter | Низкая | 1 день | Среднее |
| Инструментировать orchestrator + agent кастомными метриками | Средняя | 2-3 дня | Высокое |
| Настроить alerting (TTFT, ITL, KV cache, preemptions) | Низкая | 1 день | Высокое |
| Добавить OTel traces через пайплайн | Высокая | 3-5 дней | Высокое |
| Запустить baseline замеры латентности | Низкая | 1 день | Среднее |

### Фаза 2: Оптимизация латентности (Неделя 2-4)

| Задача | Сложность | Трудозатраты | Влияние |
|--------|----------|-------------|---------|
| Миграция Ollama → vLLM (если не завершена) | Высокая | 3-5 дней | Высокое |
| Включить prefix caching в vLLM | Низкая | 0.5 дня | Высокое |
| Уменьшить tool-calling round-trips (batch file reads) | Средняя | 2-3 дня | Высокое |
| Тюнинг `gpu_memory_utilization` и `max_model_len` | Низкая | 1 день | Среднее |
| Мониторинг Kafka consumer lag | Низкая | 1 день | Среднее |
| Добавить request-level metrics (`--enable-per-request-metrics`) | Низкая | 0.5 дня | Высокое |

### Фаза 3: Planning MVP (Неделя 3-5)

| Задача | Сложность | Трудозатраты | Влияние |
|--------|----------|-------------|---------|
| Спроектировать planning prompt для PR review | Средняя | 2-3 дня | Высокое |
| Реализовать plan parser (JSON из LLM output) | Низкая | 1-2 дня | Среднее |
| Построить step executor с Mastra tool integration | Средняя | 3-5 дней | Высокое |
| Добавить базовый reflection | Низкая | 1 день | Среднее |
| Интеграционное тестирование с реальными PR | Средняя | 2-3 дня | Высокое |
| A/B тест: planned vs unplanned reviews | Средняя | 2-3 дня | Высокое |

### Фаза 4: Production Planning (Неделя 5-8)

| Задача | Сложность | Трудозатраты | Влияние |
|--------|----------|-------------|---------|
| Добавить replanning с bounded retry (макс. 3) | Средняя | 2-3 дня | Высокое |
| Реализовать систему plan templates | Средняя | 2-3 дня | Среднее |
| Добавить step-level checkpointing (resume при сбое) | Средняя | 2-3 дня | Высокое |
| Построить judge LLM verifier | Средняя | 2-3 дня | Высокое |
| Добавить plan outcome tracking (обучение по результатам) | Высокая | 3-5 дней | Среднее |
| Полная OTel инструментация planning pipeline | Средняя | 2-3 дня | Высокое |
| Production hardening + load testing | Средняя | 2-3 дня | Высокое |

---

## Открытые вопросы

1. **Какая модель и GPU?** Это определяет, prefill или decode — доминирующий вкладчик в латентность.
2. **Сколько tool-calling round-trips в типичном ревью?** Каждый round-trip умножает общую латентность LLM.
3. **Завершена ли миграция на vLLM?** Если всё ещё на Ollama — сама миграция даёт наибольший эффект.
4. **Какой текущий consumer lag в Kafka?** Задержка очереди может быть значительным скрытым источником.
5. **Используете ли prefix caching?** Для PR review с общими system prompts это может радикально снизить TTFT.
6. **Есть ли метрики качества ревью?** Без измерения качества нельзя валидировать улучшения от планирования.

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
