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
(`ghcr.io/inno-agent/<service>`), затем по SSH деплоит на VPS
(`git checkout` нужного рефа + `docker compose pull && up -d`).

Ручной запуск (повтор / rollback): Actions → CD → Run workflow → указать `ref`
(ветка, тег или SHA). Воркфлоу пересоберёт и передеплоит именно этот коммит.

### Требуемые GitHub Actions secrets

| Secret | Значение |
|---|---|
| `SSH_HOST` | адрес VPS |
| `SSH_USER` | пользователь для SSH |
| `SSH_PRIVATE_KEY` | приватный ключ (публичный — в `~/.ssh/authorized_keys` на VPS) |
| `DEPLOY_PATH` | абсолютный путь к клону репозитория на VPS |

### Одноразовая настройка

После первого успешного запуска зайти в GitHub → организация `inno-agent` →
Packages и выставить каждому из 8 пакетов видимость **Public** — иначе
`docker compose pull` на сервере не сможет их скачать (новый package в GHCR
по умолчанию приватный).
