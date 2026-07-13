# orchestrator (innoagent)

AI-оркестратор с OpenAI-совместимым API, авто-роутингом через router-модель и поддержкой SSE-стриминга.

## Назначение

Маршрутизирует запросы на инференс LLM. Поддерживает прямой выбор модели, auto-роутинг (router-модель выбирает лучшую модель под запрос) и стриминг.

## Архитектура

```
chat-api / review-api / review-consumer
              │
              ▼
     orchestrator :8080
              │
              ▼
     Ollama :11434 (GPU или локальный)
```

## API

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/health` | Нет | Health check (статус, модель, base_url) |
| POST | `/v1/chat` | Да | Chat completion (sync/SSE) |
| GET | `/v1/models` | Да | Каталог доступных моделей |
| GET | `/metrics` | Нет | Prometheus-метрики |

### Health check

```json
{"status":"ok","model":"qwen2.5:0.5b","base_url":"http://ollama:11434/v1"}
```

### Chat (sync)

```bash
curl -X POST http://localhost:8080/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "Hello!"}],
    "model_name": "qwen2.5:0.5b",
    "stream": false
  }'
```

**Response:** `{ "answer": "..." }`

### Chat (SSE stream)

```bash
curl -X POST http://localhost:8080/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "Hello!"}],
    "model_name": "qwen2.5:0.5b",
    "stream": true
  }'
```

**Response:** SSE-поток с `data: {"answer": "..."}` чанками.

## Модели

### Список моделей

| Модель | Размер | Назначение |
|--------|--------|------------|
| `qwen2.5:0.5b` | 0.5B | Быстрая (по умолчанию) |
| `llama3.2:1b` | 1B | Универсальная |
| `qwen2.5-coder:1.5b` | 1.5B | Код |
| `fauxpaslife/arch-router:1.5b` | 1.5B | Auto-роутинг |

### Auto-роутинг

При отправке `model_name: "auto"`:
1. Router-модель (`arch-router:1.5b`) анализирует запрос
2. Выбирает лучшую модель из доступных
3. Fallback на модель по умолчанию при ошибке

### LLM_MODELS vs models.json

`LLM_MODELS` — **единственный источник истины**. Определяет:
1. Какие модели пуллятся при старте
2. Какая модель используется по умолчанию (первая в списке)
3. Что возвращает `GET /v1/models`

`internal/catalog/models.json` — **только метаданные для UI** (name, description). Не влияет на доступность моделей.

```jsonc
// models.json — metadata lookup keyed by id
{ "models": [
  { "id": "qwen2.5:0.5b", "name": "Fast", "description": "Tiny model, fastest responses" }
] }
```

Как собирается `GET /v1/models`: для каждого id из `LLM_MODELS` ищется метадата в `models.json`. Модель без записи в `models.json` всё равно отображается — name берётся из id.

- **Добавить/удалить модель** → редактировать только `LLM_MODELS`. Она пуллится, сервится, может быть default. Без пересборки кода.
- **Добавить label/description** → добавить запись в `models.json` (опционально) и пересобрать (файл эмбедится через `go:embed`).

## Конфигурация

| Переменная | Обязательна | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `LLM_BASE_URL` | Нет | `http://{OLLAMA_HOST}:{OLLAMA_PORT}/v1` | URL Ollama API |
| `OLLAMA_HOST` | Нет | `ollama` | Хост Ollama (используется если LLM_BASE_URL пуст) |
| `OLLAMA_PORT` | Нет | `11434` | Порт Ollama |
| `OLLAMA_API_KEY` | Нет | — | API-ключ Ollama |
| `LLM_MODELS` | Нет | `qwen2.5:0.5b` | Пробел-разделённый список моделей (первая = default) |
| `ROUTER_MODEL` | Нет | `fauxpaslife/arch-router:1.5b` | Модель для роутинга |
| `API_PORT` / `SERVER_PORT` | Нет | `8080` | Порт HTTP |
| `IDENTITY_URL` | Нет | `http://identity:8081` | URL identity-сервиса |

## Деплой

### Docker Compose

```bash
# Локальный Ollama
docker compose --profile local up -d

# Удалённый GPU Ollama
docker compose --profile gpu up -d

# Только оркестратор
docker compose up orchestrator
```

### Standalone (без Docker)

```bash
# Требования: Ubuntu 22.04, Go 1.26+, Ollama

# Клонирование и запуск
git clone <repo-url> /opt/innoagent
cd /opt/innoagent
cp .env.example .env
docker compose up -d

# Или без Docker
ollama serve
go run cmd/server/main.go
```

### Обновление

```bash
docker compose pull
docker compose up -d
```

## Локальная разработка

```bash
# Запуск без Docker
ollama serve
go run cmd/server/main.go

# Тесты
go test ./...
```

## Управление сервисом

```bash
# Статус
docker compose ps

# Логи
docker compose logs -f orchestrator

# Рестарт
docker compose restart orchestrator

# Остановка
docker compose down

# Полный сброс (включая кэш моделей)
docker compose down -v
```

## Траблшутинг

### Модель не загружается

```bash
docker compose logs innoagent-ollama
```

На медленном соединении загрузка большой модели может занимать несколько минут.

### Orchestrator падает с "model inference failed"

```bash
docker compose logs orchestrator
docker compose logs innoagent-ollama
```

Типичные причины:
- Загрузка модели не завершилась
- `LLM_MODELS` в `.env` не совпадает с загруженными моделями

### Порт уже занят

Измените `API_PORT` или `OLLAMA_PORT` в `.env` и перезапустите.

### Мало места на диске

Модели хранятся в Docker-volume `ollama_data`:

```bash
docker system df
docker system prune
```
