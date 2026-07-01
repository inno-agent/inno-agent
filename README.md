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
