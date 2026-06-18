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
