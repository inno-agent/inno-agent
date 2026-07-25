#!/bin/sh
set -eu

: "${TELEGRAM_BOT_TOKEN:?TELEGRAM_BOT_TOKEN is required — set it in .env}"
: "${TELEGRAM_CHAT_ID:?TELEGRAM_CHAT_ID is required — set it in .env}"

case "$TELEGRAM_CHAT_ID" in
  ''|*[!0-9-]*)
    echo "TELEGRAM_CHAT_ID must be a numeric Telegram chat id (e.g. -1001234567890), got: ${TELEGRAM_CHAT_ID}" >&2
    echo "Create a bot via @BotFather, add it to your chat, then resolve the chat id with @userinfobot or getUpdates." >&2
    exit 1
    ;;
esac

mkdir -p /alertmanager/data
if ! chown -R nobody:nobody /alertmanager/data 2>/dev/null; then
  echo "warning: could not chown /alertmanager/data — continuing (volume may already have correct ownership)" >&2
fi

sed \
  -e "s|__TELEGRAM_BOT_TOKEN__|${TELEGRAM_BOT_TOKEN}|g" \
  -e "s|__TELEGRAM_CHAT_ID__|${TELEGRAM_CHAT_ID}|g" \
  /etc/alertmanager/alertmanager.yml.template > /alertmanager/alertmanager.yml

exec /bin/alertmanager \
  --config.file=/alertmanager/alertmanager.yml \
  --storage.path=/alertmanager/data \
  --web.listen-address=:9093
