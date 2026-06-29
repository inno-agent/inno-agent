#!/bin/sh
set -eu

: "${TELEGRAM_BOT_TOKEN:?TELEGRAM_BOT_TOKEN is required — set it in .env}"
: "${TELEGRAM_CHAT_ID:?TELEGRAM_CHAT_ID is required — set it in .env}"

mkdir -p /alertmanager/data

sed \
  -e "s|__TELEGRAM_BOT_TOKEN__|${TELEGRAM_BOT_TOKEN}|g" \
  -e "s|__TELEGRAM_CHAT_ID__|${TELEGRAM_CHAT_ID}|g" \
  /etc/alertmanager/alertmanager.yml.template > /alertmanager/alertmanager.yml

exec /bin/alertmanager \
  --config.file=/alertmanager/alertmanager.yml \
  --storage.path=/alertmanager/data \
  --web.listen-address=:9093
