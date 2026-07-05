#!/bin/bash

echo "=== Проверка статуса ==="
docker compose ps promtail

echo -e "\n=== Логи Promtail (последние 10 строк) ==="
docker compose logs promtail --tail 10

echo -e "\n=== Проверка файлов логов Nginx ==="
docker compose exec proxy ls -la /var/log/nginx/ 2>/dev/null || echo "Файлы не найдены"

echo -e "\n=== Проверка доступа Promtail к логам ==="
docker compose exec promtail ls -la /var/log/nginx/ 2>/dev/null || echo "Нет доступа"

echo -e "\n=== Проверка соединения с Loki ==="
curl -s http://localhost:3100/ready && echo " ✓ Loki готов" || echo " ✗ Loki не готов"

echo -e "\n=== Проверка метрик Promtail ==="
curl -s http://localhost:9080/metrics | grep -E "promtail_(targets_active|entries_total)" | head -5

echo -e "\n=== Проверка последних записей в Loki ==="
curl -s -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query={job="nginx"}' \
  --data-urlencode 'limit=3' | jq '.data.result[]?.stream' 2>/dev/null || echo "Нет данных"

echo -e "\n=== Генерация тестовых запросов ==="
echo "Генерирую логи к эндпоинтам..."
for i in {1..3}; do
  curl -k -s https://localhost/api/v1/health > /dev/null
  curl -k -s https://localhost/llm/v1/models > /dev/null
  curl -k -s https://localhost/ > /dev/null
  echo -n "."
  sleep 0.5
done
echo " ✅ Готово!"

echo -e "\n=== Проверка после генерации ==="
curl -s -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query={job="nginx"}' \
  --data-urlencode 'limit=3' | jq '.data.result[]?.stream' 2>/dev/null || echo "Нет данных"

echo -e "\n✅ Готово! Зайди в Grafana: http://localhost:3000"