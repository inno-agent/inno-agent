# inno-agent

An AI chat assistant product, organized as a multi-language monorepo.

## Overview

A ChatGPT-style assistant with authentik-based authentication, a React frontend, a Go backend, and a Python/Ollama service for the model side.

## Repository layout

| Path | Stack | Purpose |
|---|---|---|
| `frontend/` | TypeScript, React 19, Vite, `@assistant-ui/react`, TanStack Router, Zustand, Tailwind v4, shadcn | Chat UI: SSE streaming, message branches, tool calls, markdown, reasoning blocks. Workspace split into `projects/app` (shell) and `libs/{chat,sidebar,shared}`. |
| `backend/` | Go (golangci-lint) | Identity service (gRPC + HTTP `/identity/v1/exchange`), RSA JWT issuer, Postgres user repo with migrations, authentik as OIDC provider (provisioned declaratively via blueprint in `infrastructure/authentik/`). Chat API (`backend/chat-api/`) — REST + SSE streaming, chi router, pgx, domain-driven layout. |
| `ai/` | Python 3.14, uv, ruff, mypy, pytest | LLM-serving module. Dockerized Ollama deployment in progress. |

## CI & tooling

- GitHub Actions: `ci-backend.yml`, `ci-frontend.yml`, `ci-python.yml`
- Lefthook pre-commit: gofumpt + golangci-lint (Go), ruff + mypy (Python)
- Pre-push hooks currently disabled

## Active development branches

| Branch | Focus |
|---|---|
| `authorization/zitadel-prettify` | Auth service — gRPC, JWT, Zitadel, Terraform (актуальная) |
| `chat-setup` | Chat API — Go REST + SSE, domain-driven, Postgres |
| `feature/ollama-docker` | Local LLM (Ollama) deployment |
| `web-chat` | Frontend chat UI fixes |
| `kemvk06/innoai-22-…` | Logged-in main page layout (Tracker: `INNOAI-22`) |

---

# 📖 Инструкция по работе с проектом
 
## 1. Установка Lefthook
 
[Lefthook](https://github.com/evilmartians/lefthook) — менеджер git-хуков.
 
### Установка
 
**Homebrew, apt, winget** 
```bash
brew install lefthook
# или
sudo apt install lefthook
# или
winget install -e --id evilmartians.lefthook
``` 
Другой: https://lefthook.dev/install
### Активация хуков
 
```bash
lefthook install
```
 
Это зарегистрирует хуки из `lefthook.yml` в локальном `.git/hooks`.
