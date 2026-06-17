# InnoAgent

AI orchestrator service built on Go + Ollama + Qwen, deployable to any Ubuntu 22.04 server with a single command.

## Quick Start (Docker already installed)

```bash
git clone <your-repo-url>
cd innoagent
cp .env.example .env
docker compose up -d
```

Test:

```bash
curl http://localhost:8080/health
```

---

## Architecture

```
Client
  ↓  POST /chat
Orchestrator (Go, port 8080)
  ↓  Simple chat API
Ollama (port 11434)
  ↓
Qwen model (local)
```

---

## Prerequisites

- Ubuntu 22.04 server (fresh install is fine)
- `sudo` access
- Internet access (to pull Docker images and model)

---

## Fresh Server Deployment

```bash
git clone <your-repo-url> /opt/innoagent
cd /opt/innoagent
sudo bash install.sh
```

`install.sh` will:
1. Install Docker and Docker Compose
2. Configure the firewall (ports 22, 8080)
3. Copy `.env.example` → `.env` if not present
4. Pull base images, build, and start all services

---

## Configuration

Edit `.env` before starting if you want to change model/hostname/ollama port/api port:

| Variable | Default | Description |
|---|---|---|
| `LLM_MODELS` | `qwen2.5:0.5b llama3.2:1b qwen2.5-coder:1.5b` | Space-separated models; first is the default |
| `OLLAMA_HOST` | `ollama` | Ollama hostname (inside Docker network) |
| `OLLAMA_PORT` | `11434` | Ollama port exposed on host |
| `API_PORT` | `8080` | Orchestrator API port exposed on host |

---

## Local Development

Run without Docker:

```bash
ollama serve
go run cmd/server/main.go
```

Run tests:

```bash
go test ./...
```

---

## API

### Health check

```bash
curl http://localhost:8080/health
```

```json
{"status":"ok","model":"qwen2.5:0.5b","base_url":"http://ollama:11434/v1"}
```

### Chat

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"What is the capital of France?"}'
```

```json
{"answer":"The capital of France is Paris."}
```

---

## Update Deployment

```bash
cd /opt/innoagent
bash deploy.sh
```

`deploy.sh` will:
1. Pull latest git changes
2. Pull latest Ollama base image
3. Rebuild the orchestrator image (no cache)
4. Restart all services

---

## Verification

```bash
bash verify.sh
```

Checks:
- Ollama is reachable
- Model is available
- Orchestrator `/health` responds
- Chat endpoint returns a valid response

---

## Service Management

```bash
# View status
docker compose ps

# View logs
docker compose logs -f orchestrator
docker compose logs -f innoagent-ollama

# Restart a service
docker compose restart orchestrator

# Stop everything
docker compose down

# Stop and remove all data (including model cache)
docker compose down -v
```

---

## Troubleshooting

### Model not loading

```bash
docker compose logs innoagent-ollama
```

On a slow connection a large model can take several minutes to pull.

### Orchestrator exits with "model inference failed"

```bash
docker compose logs orchestrator
docker compose logs innoagent-ollama
```

Common causes:
- Model pull did not complete
- `LLM_MODELS` in `.env` does not match what was pulled

### Port already in use

Change `API_PORT` or `OLLAMA_PORT` in `.env` then:

```bash
docker compose up -d
```

### Out of disk space

Model files are stored in the `ollama_data` Docker volume. Check with:

```bash
docker system df
```

Remove unused data:

```bash
docker system prune
```

### Resetting everything

```bash
docker compose down -v
docker compose up -d
```

This removes the model cache; the model will be pulled again on startup.

---

## Important Files

| File | Purpose |
|---|---|
| `docker-compose.yml` | Service definitions |
| `Dockerfile` | Go orchestrator build |
| `.env` | Runtime configuration |
| `.env.example` | Configuration template |
| `install.sh` | Bootstrap fresh Ubuntu server |
| `deploy.sh` | Pull updates and rebuild |
| `verify.sh` | End-to-end stack health check |
