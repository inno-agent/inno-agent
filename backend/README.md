# Inno-Agent Backend

Go-монорепозиторий с шестью сервисами, объединёнными через `go.work`. Все сервисы — самостоятельные бинарники с собственными Dockerfile, базами данных и миграциями.

## Структура

```
backend/
├── go.work                    # Go workspace (go 1.26.4)
├── .golangci.yml              # Linter-конфигурация (govet, staticcheck, errcheck, gosec)
│
├── identity/                  # Централизованная авторизация (JWT + OIDC)
├── chat-api/                  # Чат API с SSE-стримингом
├── innoagent/                 # AI-оркестратор (LLM-роутинг + инференс)
├── review-api/                # Ручной PR-ревью + онбординг
├── review-consumer/           # Kafka-consumer для автоматических PR-ревью
├── review-webhook/            # Webhook-приёмник → Kafka-продюсер
│
└── pkg/
    └── telemetry/             # Общая библиотека метрик (Prometheus)
```

## Архитектура

### Chat Pipeline

```
┌──────────┐    HTTPS     ┌──────────┐   POST /v1/chat    ┌────────────┐   OpenAI API    ┌────────┐
│ chat-fe  │──── :9443 ──▶│ chat-api │──── (SSE) ────────▶│orchestrator│──── /v1/chat ──▶│ Ollama │
│ React    │              │  :8000   │                     │   :8080    │  /completions   │:11434  │
└──────────┘              └──────────┘                     └─────┬──────┘                 └────────┘
                                │                                 │
                                │ validate JWT                    │ validate JWT
                                ▼                                 ▼
                          ┌──────────┐                    ┌────────────┐
                          │identity  │◀────── OIDC ──────│ Authentik  │
                          │  :8081   │       JWKS         │   :443     │
                          └──────────┘                    └────────────┘
```

### Review Pipeline

```
┌─────────┐  webhook  ┌─────────────┐  publish  ┌───────────┐  consume  ┌─────────────────┐
│ GitFlame │─────────▶│review-webhook│─────────▶│  Redpanda │─────────▶│review-consumer   │
│ (forge)  │          │    :8002    │           │  (Kafka)  │           │  :9090 (metrics) │
└────┬─────┘          └─────────────┘           │  :9092    │           └────────┬────────┘
     │                                          └───────────┘                    │
     │                                                                   ┌──────┴──────┐
     │                                                         primary   │             │
     │                                                         (planned) │      fallback│
     │                                                                   ▼             ▼
     │                                                         ┌──────────────┐  ┌────────────┐
     │                                                         │review-agent  │  │orchestrator│
     │                                                         │  (Mastra)    │  │   :8080    │
     │                                                         │ tool calling │  └─────┬──────┘
     │                                                         │ [WIP]        │        │
     │                                                         └──────┬───────┘        │
     │                                                                │                │
     │                                                                ▼                ▼
     │                                                          POST /v1/chat    ┌──────────┐
     │                                                              ─────────────▶│  Ollama  │
     │                                                                           │  :11434  │
     │                                                                           └──────────┘
     │
     └── review-consumer ──GET diff/comment──▶ GitFlame (outbound API)
     └── review-consumer ──service-token─────▶ identity (RFC 8693 token exchange)
```

### Data Layer

```
┌──────────────────────────────────────────────────────────────┐
│                     PostgreSQL :5432                          │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐ ┌──────────────────┐  │
│  │authentik │ │inno_auth │ │llm_chat │ │   inno_review    │  │
│  │  (IdP)   │ │(identity)│ │(chat-api│ │ (review-api +    │  │
│  │          │ │          │ │         │ │  review-consumer) │  │
│  └──────────┘ └──────────┘ └─────────┘ └──────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## Сервисы

| Сервис | Порт | Фреймворк | База данных | Назначение |
|--------|------|-----------|-------------|------------|
| [identity](#identity) | 8081 | Gin | `inno_auth` | JWT-эмиттер, OIDC-мост, токен-менеджмент |
| [chat-api](#chat-api) | 8000 | Chi | `llm_chat` | Чат-сессии, SSE-стриминг LLM-ответов |
| [orchestrator (innoagent)](#orchestrator) | 8080 | net/http | — | LLM-роутинг, OpenAI-совместимый API |
| [review-api](#review-api) | 8001 | Chi | `inno_review` | Ручной AI-ревью PR, онбординг пользователей |
| [review-consumer](#review-consumer) | 9090 (метрики) | kafka-go | `inno_review` | Автоматические PR-ревью по webhook-событиям |
| [review-webhook](#review-webhook) | 8002 | Chi | — | Приём webhook-ов GitFlame → Kafka |
| [pkg/telemetry](#pkgtelemetry) | — | — | — | Общая Prometheus-библиотека |

## Базы данных

Один экземпляр PostgreSQL с 4 базами и 4 ролями:

| База данных | Владелец | Сервис | Назначение |
|-------------|----------|--------|------------|
| `authentik` | `authentik` | Authentik | Данные OIDC-провайдера |
| `inno_auth` | `identity` | identity | JWT, пользователи, сервис-клиенты, delegation-гранты |
| `llm_chat` | `chat` | chat-api | Чаты, сообщения |
| `inno_review` | `review` | review-api, review-consumer | Привязки gitflame_username → user_id |

Миграции выполняются контейнерами `migrate/migrate:latest` при старте (job-ы с `restart: "no"`).

## Go Workspace

```go
// go.work
go 1.26.4

use (
    ./chat-api
    ./identity
    ./innoagent
    ./pkg/telemetry
    ./review-api
    ./review-consumer
    ./review-webhook
)
```

Каждый сервис использует `replace` для `pkg/telemetry` через относительный путь `../pkg/telemetry`.

## identity

Централизованный сервис авторизации: JWT-эмиттер (RS256), OIDC-интеграция с Authentik, refresh-токены с reuse detection, delegation-гранты для service-to-service аутентификации.

### API

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/identity/v1/config` | OIDC-конфигурация для фронтенда |
| GET | `/identity/v1/jwks` | Публичные JWKS-ключи |
| POST | `/identity/v1/validate` | Валидация JWT → user_id |
| POST | `/identity/v1/exchange` | Обмен OIDC-токена на JWT |
| POST | `/identity/v1/refresh` | Ротация refresh-токена |
| POST | `/identity/v1/revoke` | Отзыв refresh-токена |
| POST | `/identity/v1/service-token` | Сервисные credentials → JWT |
| POST | `/identity/v1/delegation-grant` | Создание delegation-гранта |
| POST | `/identity/v1/token` | RFC 8693 token exchange |
| GET | `/metrics` | Prometheus-метрики |

### Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `OIDC_ISSUER` | Да | — | URL OIDC-провайдера (Authentik) |
| `OIDC_JWKS_URL` | Нет | `{OIDC_ISSUER}/jwks/` | Внутренний URL для JWKS |
| `OIDC_CLIENT_ID` | Да | — | OIDC client ID |
| `AUTH_JWT_PRIVATE_KEY_PATH` | Да | — | Путь к RSA-приватному ключу |
| `AUTH_DATABASE_DSN` | Да | — | PostgreSQL DSN |
| `AUTH_HTTP_PORT` | Нет | `8081` | Порт HTTP |
| `AUTH_JWT_EXPIRY` | Нет | `30m` | TTL access-токена |
| `AUTH_REFRESH_EXPIRY` | Нет | `720h` | TTL refresh-токена |
| `SERVICE_TOKEN_EXPIRY` | Нет | `1h` | TTL сервисного токена |
| `DELEGATE_TOKEN_EXPIRY` | Нет | `15m` | TTL делегированного токена |
| `SEED_CLIENT_ID` | Нет | — | Авто-создание сервис-клиента при старте |
| `SEED_CLIENT_SECRET` | Нет | — | Секрет для seed-клиента |

### Зависимости

- PostgreSQL (`inno_auth`)
- Authentik (OIDC-провайдер)
- RSA-ключ (Docker secret: `identity_rsa_key`)

---

## chat-api

Пользовательский API для управления чат-сессиями и стриминга ответов LLM через SSE.

### API

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/health` | Нет | Health check |
| GET | `/api/v1/chats` | Да | Список чатов (пагинация) |
| GET | `/api/v1/chats/{chat_id}/messages` | Да | Сообщения чата |
| POST | `/api/v1/chats/{chat_id}/stream` | Да | SSE-стриминг ответа LLM |
| DELETE | `/api/v1/chats/{chat_id}` | Да | Мягкое удаление чата |
| GET | `/metrics` | Нет | Prometheus-метрики |

### SSE-события

```
event: status
data: {"status": "context_loading"}

event: chunk
data: {"chunk": "Hello"}

event: done
data: {"done": true}
```

### Middleware

CorrelationID → Logger → RequestLogger → CORS → Auth (только `/api/v1`) → Telemetry

### Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `DATABASE_URL` | Да | — | PostgreSQL DSN |
| `SERVER_PORT` | Нет | `8000` | Порт HTTP |
| `ORCHESTRATOR_URL` | Нет | `http://orchestrator:8080` | URL оркестратора |
| `AUTH_SERVICE_URL` | Нет | `http://identity:8081` | URL identity-сервиса |
| `READ_TIMEOUT` | Нет | `10s` | HTTP read timeout |
| `WRITE_TIMEOUT` | Нет | `0` (без ограничений) | HTTP write timeout |
| `IDLE_TIMEOUT` | Нет | `120s` | HTTP idle timeout |

### Зависимости

- PostgreSQL (`llm_chat`)
- orchestrator (LLM-инференс)
- identity (валидация токенов)

---

## orchestrator (innoagent)

AI-оркестратор с OpenAI-совместимым API. Поддерживает auto-routing через router-модель, прямой выбор модели и стриминг.

### API

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/health` | Нет | Health check (статус, модель, base_url) |
| POST | `/v1/chat` | Да | Chat completion (sync/SSE) |
| GET | `/v1/models` | Да | Каталог доступных моделей |
| GET | `/metrics` | Нет | Prometheus-метрики |

### Модели

| Модель | Размер | Назначение |
|--------|--------|------------|
| `qwen2.5:0.5b` | 0.5B | Быстрая (по умолчанию) |
| `llama3.2:1b` | 1B | Универсальная |
| `qwen2.5-coder:1.5b` | 1.5B | Код |
| `fauxpaslife/arch-router:1.5b` | 1.5B | Auto-роутинг |

### Auto-роутинг

При отправке `model_name: "auto"`:
1. Router-модель анализирует запрос
2. Выбирает лучшую модель из доступных
3. Fallback на модель по умолчанию при ошибке

### Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `LLM_BASE_URL` | Нет | `http://{OLLAMA_HOST}:{OLLAMA_PORT}/v1` | URL Ollama API |
| `OLLAMA_HOST` | Нет | `ollama` | Хост Ollama |
| `OLLAMA_PORT` | Нет | `11434` | Порт Ollama |
| `OLLAMA_API_KEY` | Нет | — | API-ключ Ollama |
| `LLM_MODELS` | Нет | `qwen2.5:0.5b` | Пробел-разделённый список моделей (первая = default) |
| `ROUTER_MODEL` | Нет | `fauxpaslife/arch-router:1.5b` | Модель для роутинга |
| `API_PORT` / `SERVER_PORT` | Нет | `8080` | Порт HTTP |
| `IDENTITY_URL` | Нет | `http://identity:8081` | URL identity-сервиса |

### models.json vs LLM_MODELS

`LLM_MODELS` — единственный источник истины. Определяет:
1. Какие модели пуллятся при старте
2. Какая модель используется по умолчанию (первая в списке)
3. Что возвращает `GET /v1/models`

`internal/catalog/models.json` — только метаданные для UI (name, description). Не влияет на доступность моделей.

### Зависимости

- Ollama (LLM-бэкенд)
- identity (валидация токенов)

---

## review-api

API для ручного AI-ревью PR и онбординга пользователей (привязка GitFlame-аккаунта).

### API

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/health` | Нет | Health check |
| POST | `/api/v1/review` | Да | Генерация AI-ревью |
| POST | `/api/v1/installations` | Да | Привязка GitFlame-аккаунта |
| GET | `/api/v1/installations/me` | Да | Получение привязанного аккаунта |
| POST | `/api/v1/invitations/accept` | Да | Принятие приглашения |
| GET | `/metrics` | Нет | Prometheus-метрики |

> Эндпоинты installations и invitations доступны только если задан `REVIEW_DATABASE_DSN`.

### Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `SERVER_PORT` | Нет | `8001` | Порт HTTP |
| `ORCHESTRATOR_URL` | Нет | `http://orchestrator:8080` | URL оркестратора |
| `AUTH_SERVICE_URL` | Нет | `http://identity:8081` | URL identity-сервиса |
| `GITFLAME_BASE_URL` | Нет | — | URL API GitFlame |
| `GITFLAME_TOKEN` | Нет | — | PAT GitFlame |
| `REVIEW_DATABASE_DSN` | Нет | — | PostgreSQL DSN (включает онбординг-роуты) |
| `REVIEW_CONSUMER_CLIENT_ID` | Нет | `review-consumer` | Service client для delegation-грантов |

### Зависимости

- PostgreSQL (`inno_review`) — опционально, для онбординга
- orchestrator (LLM-инференс)
- identity (авторизация + delegation)
- GitFlame (diff, invites)

---

## review-consumer

Асинхронный воркер для автоматических PR-ревью. Потребляет события из Kafka, генерирует AI-ревью, публикует результат в GitFlame.

### Поток обработки

```
Kafka event → Filter (reviewer_added) → Dedup → Get diff
→ Get context files (AGENTS.md, README.md) → Get delegated JWT (RFC 8693)
→ Call LLM → Post PR comment → Remove self as reviewer
```

### Архитектура

```
Kafka (gitflame.events) ──▶ review-consumer ──▶ orchestrator (LLM)
                                                ├── GitFlame API (diff, comments)
                                                └── identity (token exchange)
```

### Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `KAFKA_BROKERS` | Нет | `redpanda:9092` | Kafka-брокер |
| `KAFKA_TOPIC` | Нет | `gitflame.events` | Kafka-топик |
| `KAFKA_GROUP` | Нет | `review-consumer` | Consumer group |
| `ORCHESTRATOR_URL` | Нет | `http://orchestrator:8080` | URL оркестратора |
| `ORCHESTRATOR_TOKEN` | Нет | — | Статический токен для оркестратора |
| `REVIEW_MODEL` | Нет | `qwen2.5-coder:1.5b` | Модель для ревью |
| `GITFLAME_BASE_URL` | Нет | — | URL API GitFlame |
| `GITFLAME_TOKEN` | Нет | — | PAT GitFlame |
| `BOT_GITFLAME_USERNAME` | Нет | — | GitFlame-имя бота |
| `IDENTITY_URL` | Нет | `http://identity:8081` | URL identity-сервиса |
| `REVIEW_DATABASE_DSN` | Нет | — | PostgreSQL DSN |
| `REVIEW_SERVICE_CLIENT_ID` | Нет | `review-consumer` | Service client ID |
| `REVIEW_SERVICE_CLIENT_SECRET` | Нет | — | Service client secret |

### Зависимости

- Kafka (Redpanda, топик `gitflame.events`)
- PostgreSQL (`inno_review`)
- orchestrator (LLM-инференс)
- identity (токен-обмен)
- GitFlame (diff, комментарии, удаление ревьюера)

---

## review-webhook

Приём webhook-ов от GitFlame и публикация событий в Kafka.

### API

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/healthz` | Нет | Health check |
| POST | `/hooks/gitflame` | Да | Приём GitFlame webhook |
| GET | `/metrics` | Нет | Prometheus-метрики |

### Безопасность

- Валидация `Authorization` header с constant-time сравнением
- Пустое `WEBHOOK_AUTHORIZATION` отключает проверку (dev-режим)
- Оборачивает payload в envelope (delivery_id, event_type)
- Партиционирование по репозиторию в Kafka

### Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `SERVER_PORT` | Нет | `8002` | Порт HTTP |
| `KAFKA_BROKERS` | Нет | `redpanda:9092` | Kafka-брокер |
| `KAFKA_TOPIC` | Нет | `gitflame.events` | Kafka-топик |
| `WEBHOOK_AUTHORIZATION` | Нет | — | Ожидаемое значение Authorization |
| `WEBHOOK_AUTH_HEADER` | Нет | `Authorization` | Имя заголовка для auth |
| `WEBHOOK_EVENT_HEADER` | Нет | `X-GitFlame-Event` | Имя заголовка с типом события |
| `WEBHOOK_DELIVERY_HEADER` | Нет | `X-GitFlame-Delivery` | Имя заголовка с delivery ID |

### Зависимости

- Kafka (Redpanda)

---

## pkg/telemetry

Общая библиотека метрик для всех Go-сервисов. Prometheus-регистрация, HTTP-middleware для Chi/Gin/net/http, standalone-metrics-сервер.

### Использование

```go
import "github.com/inno-agent/inno-agent/backend/pkg/telemetry"

// Инициализация
telemetry.Init("my-service")

// Chi middleware
r.Use(telemetry.ChiMiddleware("my-service"))

// Gin middleware
r.Use(telemetry.GinMiddleware("my-service"))

// net/http middleware
handler = telemetry.StdMiddleware("my-service", next)

// Metrics endpoint
telemetry.Handler() // promhttp.HandlerFor (OpenMetrics)

// Standalone-сервер (для воркеров без HTTP)
telemetry.ListenAndServe(":9090", "my-service")
```

### Метрики

| Метрика | Тип | Описание |
|---------|-----|----------|
| `service_http_requests_total` | Counter | Общее число HTTP-запросов |
| `service_http_request_duration_seconds` | Histogram | Латентность запросов |
| `service_http_requests_in_flight` | Gauge | Текущие in-flight запросы |
| `service_errors_total` | Counter | Общее число ошибок |
| `service_up` | Gauge | Health сервиса (1=up, 0=down) |
| `service_healthcheck_total` | Counter | Число healthcheck-проб |

### Алиасы по сервисам

| Сервис | Префикс |
|--------|---------|
| chat-api | `chat` |
| review-api | `review` |
| review-webhook | `webhook` |
| review-consumer | `consumer` |
| orchestrator | `orchestrator` |
| identity | `identity` |

---

## Тесты

28 тестовых файлов покрывают все сервисы:

| Сервис | Тесты | Покрытие |
|--------|-------|----------|
| chat-api | handler/chats, messages, stream; llm/client; repository/delete | HTTP-хендлеры, LLM-клиент, удаление |
| identity | transport/http, user/repository, issuer, provider/oidc, config | HTTP-роуты, репозиторий, JWT, OIDC |
| review-api | handler/review, installation, invite; service; gitflame/client | Хендлеры, сервис, GitFlame-клиент |
| innoagent | orchestrator, llm/qwen, catalog, auth/middleware | Роутинг, провайдеры, каталог, auth |
| review-consumer | processor, bounded_set, tokensource, gitflame, review, event | Обработка событий, дедуп, токены |
| review-webhook | webhook/handler, kafka/publisher | Webhook-хендлер, Kafka-публикация |
| pkg/telemetry | flusher_test | Flush для SSE |

Запуск:

```bash
# Все сервисы
cd backend && go test ./...

# Один сервис
cd backend/chat-api && go test ./...
```

---

## Локальная разработка

```bash
# 1. Без Docker
cd backend
go run chat-api/cmd/server/main.go
go run identity/cmd/server/main.go
go run innoagent/cmd/server/main.go

# 2. С Docker
docker compose up -d

# 3. Только локальный Ollama
docker compose --profile local up -d

# 4. С удалённым GPU Ollama
docker compose --profile gpu up -d
```

## CI/CD

| Workflow | Триггер | Описание |
|----------|---------|----------|
| `ci-backend.yml` | PR/main (backend/) | Go тесты + lint |
| `cd.yml` | Push to main | Build → Push GHCR → Deploy на agr01 |

### CI Backend

Для каждого сервиса с `go.mod`: `go test -race ./...` + `go build ./...` + `golangci-lint run`.

### CD

Матричная сборка 8 сервисов → пуш в `ghcr.io/inno-agent/{service}:{sha}` → деплой на self-hosted раннер `agr01` через `scripts/prod.sh`.

## Дополнительная документация

- [Документация backend (docs/backend/)](../docs/backend/README.md) — структурированная документация по всем сервисам
- [API Reference](../docs/backend/api-reference.md) — все HTTP-эндпоинты
- [Database](../docs/backend/database.md) — PostgreSQL-схемы
- [Authentication](../docs/backend/auth.md) — Auth-flow, JWT, token exchange
- [Deployment](../docs/backend/deployment.md) — Docker, env vars, production
