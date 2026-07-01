# 📖 Инструкция по работе с проектом

## 1. Установка Lefthook

[Lefthook](https://github.com/evilmartians/lefthook) — менеджер git-хуков.

### Установка

| OS | Команда |
|---|---|
| macOS | `brew install lefthook` |
| Linux / WSL | `sudo apt install lefthook` |
| Windows (PowerShell) | `winget install -e --id evilmartians.lefthook` |

Другие варианты: <https://lefthook.dev/install>

### Активация хуков

```bash
lefthook install
```

Это зарегистрирует хуки из `lefthook.yml` в локальном `.git/hooks`.

## 2. Запуск проекта

**Пререквизит — [mkcert](https://github.com/FiloSottile/mkcert)** (доверенный локальный CA для того, чтобы браузеры доверяли <https://localhost>):

| OS | Команда |
|---|---|
| macOS | `brew install mkcert nss` |
| Linux / WSL | `apt install mkcert libnss3-tools` |
| Windows (PowerShell) | `winget install -e --id FiloSottile.mkcert` |

Затем:

```bash
./scripts/dev-setup.sh
docker compose up -d --build
```

> Windows: sh-скрипты запускать в Git Bash!

> Сброс с нуля: `docker compose down -v`, затем снова `./scripts/dev-setup.sh && docker compose up -d --build`.
> Firefox требует `certutil` (nss), иначе mkcert не пропишет CA в его стор.

- Приложение: `https://localhost`
- PR-ревьюер: `https://review.localhost`
- Authentik (вход / админка): `https://auth.localhost`

⚠️ **SMTP-переменные (`AUTHENTIK_EMAIL__*`) в `.env` обязательны** — саморегистрация
активирует аккаунт письмом-подтверждением; без SMTP новые пользователи останутся
неактивными. Тем не менее, можно заходить под админским доступом: akadmin:$AUTHENTIK_ADMIN_PASSWORD

## 3. Деплой (CD)

Пуш в `main` гонит `.github/workflows/cd.yml`: собирает 8 сервисов, пушит в GHCR
(`ghcr.io/inno-agent/<service>`), затем джоба `deploy` выполняется прямо на
self-hosted раннере `agr01`, который живёт на проде: логинится в GHCR через
встроенный `GITHUB_TOKEN` и гонит `scripts/prod.sh TAG=<sha>`. Публичность
GHCR-пакетов и SSH-секреты не нужны — раннер и есть прод-хост.

`scripts/prod.sh` (идемпотентный, как `dev-setup.sh` для дева):
генерирует `backend/identity/dev-private-key.pem` (JWT-ключ) и
самоподписанный TLS-серт в `infrastructure/nginx/certs/` — но только если их
там ещё нет; дальше пишет `TAG` в `~/.env.innoagent` и гонит
`docker compose --profile gpu pull && up -d`. Можно гонять и руками на
раннере (`TAG=latest ./scripts/prod.sh`) для редеплоя без CI.

Ручной запуск через Actions (повтор / rollback): Actions → CD → Run workflow
→ указать `ref` (ветка, тег или SHA). Воркфлоу пересоберёт и передеплоит
именно этот коммит.

### Self-hosted раннер (одноразовая настройка на VPS)

- Зарегистрировать GitHub Actions runner с лейблами `self-hosted, agr01` в
  этом репозитории, юзер раннера должен иметь доступ к `docker`.
- Раннер переиспользует один и тот же рабочий каталог между запусками — это
  и есть каталог деплоя (`docker-compose.yml` там же, где чекаутится репо).
  Checkout деплой-джобы идёт с `clean: false` — специально, чтобы не сносить
  gitignored рантайм-файлы (`backend/identity/dev-private-key.pem`, `.ollama/`,
  прод-сертификаты `infrastructure/nginx/certs/*.pem`) при каждом деплое.
- `backend/identity/dev-private-key.pem` и `infrastructure/nginx/certs/*.pem`
  руками класть не надо — `prod.sh` сам сгенерит при первом запуске, если их
  там ещё нет (серт — самоподписанный на `AUTH_DOMAIN` из `~/.env.innoagent`,
  браузер будет ругаться — осознанно, сервер во внутренней сети).
- `~/.env.innoagent` (в домашней директории раннера, вне репо) — скопировать
  руками из `.env.prod`. Полный набор переменных как в `.env.example`, плюс:
  - `OLLAMA_BASE_URL` — прод использует `--profile gpu` (внешний GPU Ollama),
    заполнить реальным адресом (пример закомментирован в `.env.example`).
  - `GRAFANA_ADMIN_PASSWORD` — переопределить дефолтный `admin` из
    `.env.example`.
  - `TAG` пишет туда сама CD-джоба при каждом деплое, руками не трогать.
- Grafana (`3000`), Loki (`3100`) и `review-api` (`8001`) забинжены на
  `127.0.0.1` — наружу не торчат, доступ только через SSH-туннель или с самого
  хоста.
