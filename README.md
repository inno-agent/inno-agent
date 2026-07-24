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

- Authentik (вход / админка): `https://localhost` (443, дефолт — без поддомена,
  ему так проще: сам строит свои API/issuer URL из Host без порта)
- Чат-приложение (`frontend`, код в `frontend/chat-front`): `https://chat.localhost:9443`
- PR-ревьюер (`review-front`, код в `frontend/review-front`): `https://review.localhost:8443`

Поддомены `chat.`/`review.` — просто для читаемости локально (mkcert их
покрывает); реально роутит порт, не hostname. На проде (голый IP, без DNS)
поддоменов физически нет — там различают только порты, см. раздел 3.

⚠️ **SMTP-переменные (`AUTHENTIK_EMAIL__*`) в `.env` обязательны** — саморегистрация
активирует аккаунт письмом-подтверждением; без SMTP новые пользователи останутся
неактивными. Тем не менее, можно заходить под админским доступом: akadmin:$AUTHENTIK_ADMIN_PASSWORD

## 3. Деплой (CD)

Пуш в `main` гонит `.github/workflows/cd.yml`: собирает образы, пушит в GHCR
(`ghcr.io/inno-agent/<service>`), затем джоба `deploy-dev` выполняется на
self-hosted раннере `agr01` и гонит `scripts/dev.sh TAG=<sha>`. **Prod из
GitHub Actions не обновляется** — только dev.

Promote dev → prod: авторизованный пользователь заходит на сервер по SSH и
гонит `./scripts/promote.sh` из каталога деплоя. Скрипт берёт `TAG` из
`~/.env.innoagent.dev` и деплоит тот же коммит в prod через `scripts/prod.sh`.
Кто может — решается доступом на сервер, не GitHub.

### Dev vs prod на одной машине

Два изолированных стека: `innoagent-dev` (CD) и `inno-agent` (prod, как раньше).
Свои env-файлы, volumes, networks, identity-ключ и TLS-серты.

| | DEV | PROD |
|---|---|---|
| Env | `~/.env.innoagent.dev` | `~/.env.innoagent` |
| Compose | `docker-compose.yml` + `docker-compose.dev.yml` | `docker-compose.yml` |
| Deploy | `scripts/dev.sh` (CD) | `scripts/promote.sh` → `prod.sh` |
| GPU profile | нет (remote Ollama) | `--profile gpu` |

Порты dev = prod + 1 на последней цифре:

| Сервис | PROD | DEV |
|---|---|---|
| Authentik | `https://<IP>` (443) | `https://<IP>:4443` |
| Chat | `:9443` | `:9444` |
| Review | `:8443` | `:8444` |
| Grafana (localhost) | 3000 | 3001 |

`scripts/dev.sh` (идемпотентный): генерирует
`backend/identity/dev-private-key.dev.pem` и серт в
`infrastructure/nginx/certs-dev/` — только если их ещё нет; пишет `TAG` в
`~/.env.innoagent.dev`; `docker compose pull && up -d` без `--profile gpu`.

`scripts/prod.sh` (идемпотентный): как раньше — ключ в
`backend/identity/dev-private-key.pem`, серт в `infrastructure/nginx/certs/`,
`docker compose --profile gpu pull && up -d`.

Ручной dev-деплой через Actions (повтор / rollback): Actions → CD → Run workflow
→ указать `ref`. Prod — только `./scripts/promote.sh` с сервера.

### Self-hosted раннер (одноразовая настройка на VPS)

- Зарегистрировать GitHub Actions runner с лейблами `self-hosted, agr01` в
  этом репозитории, юзер раннера должен иметь доступ к `docker`.
- Раннер переиспользует один и тот же рабочий каталог между запусками — это
  и есть каталог деплоя (`docker-compose.yml` там же, где чекаутится репо).
  Checkout деплой-джобы идёт с `clean: false` — специально, чтобы не сносить
  gitignored рантайм-файлы (`backend/identity/dev-private-key*.pem`, `.ollama/`,
  сертификаты `infrastructure/nginx/certs*/`) при каждом деплое.
- Prod runtime-файлы (`dev-private-key.pem`, `certs/*.pem`) — `prod.sh` сам
  сгенерит при первом запуске, если их ещё нет.
- Dev runtime-файлы (`dev-private-key.dev.pem`, `certs-dev/*.pem`) — `dev.sh`
  аналогично.
- `~/.env.innoagent` (prod) — скопировать руками из `.env.prod`. Полный набор
  переменных как в `.env.example`, плюс:
  - `OLLAMA_BASE_URL` — прод использует `--profile gpu` (внешний GPU Ollama),
    заполнить реальным адресом (пример закомментирован в `.env.example`).
  - `GRAFANA_ADMIN_PASSWORD` — переопределить дефолтный `admin` из
    `.env.example`.
  - `TAG` пишет promote/CD, руками не трогать.
- `~/.env.innoagent.dev` (dev) — копия prod-env с dev-портами в URL:
  - `AUTH_ISSUER_URL=https://<IP>:4443`
  - `APP_CALLBACK_URL=https://<IP>:9444/callback`
  - `REVIEW_CALLBACK_URL=https://<IP>:8444/callback`
  - `OLLAMA_BASE_URL` — remote GPU (без `--profile gpu` на dev)
  - `ONBOARDING_URL=https://<IP>:8444` (если используется)
- Grafana/Loki/review-api на prod: `3000`/`3100`/`8001` на `127.0.0.1`; на
  dev — `3001`/`3101`/`8002`.

### Домен на проде: нет домена, нет проброса — только внутренняя сеть

Сервер сидит на приватном IP (например `10.100.32.36`), без DNS и без
проброса порта от универа наружу — снаружи (не из VPN/кампуса) не
достучаться физически, это отдельный сетевой вопрос, не про CD.
`nip.io`-трюк для поддоменов не взлетел: кампусный DNS-фильтр перехватывает
`*.nip.io` как DNS-rebinding risk и подсовывает левый IP при резолве изнутри
сети.

Поэтому вместо поддоменов (`auth.`/`review.`) — порты на одном и том же
хосте:

| Сервис | Адрес |
|---|---|
| Authentik | `https://<AUTH_DOMAIN>` (443, дефолт) |
| Chat-приложение | `https://<AUTH_DOMAIN>:9443` |
| PR-ревьюер | `https://<AUTH_DOMAIN>:8443` |

Authentik — на дефолтном порту специально: он сам генерит свои абсолютные
API/issuer URL из полученного `Host`-заголовка (nginx форвардит его через
`$http_host`, с портом как есть) — на дефолтном порту в `Host` порта нет
вообще, и нечему рассинхронизироваться с тем, что реально видит браузер.

`AUTH_DOMAIN` в `.env.prod` — голый IP (`10.100.32.36`), без dashes/nip.io.
Плюс три явных URL-переменных (замена прежней сборки из `AUTH_DOMAIN` +
`auth.`/`review.` префиксов, которая на голом IP не работает):

- `AUTH_ISSUER_URL` — куда реально смотрит браузер при заходе в Authentik
  (без порта — см. выше).
- `APP_CALLBACK_URL` / `REVIEW_CALLBACK_URL` — OAuth `redirect_uri` для
  чата и ревьюера (тут порт **нужен** — они не на 443, так его реально
  шлёт браузер в `redirect_uri`).

Если позже дадут публичный IP/домен — поменять только эти переменные в
`~/.env.innoagent`, порты/код трогать не надо.
