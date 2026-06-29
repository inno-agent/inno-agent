module github.com/inno-agent/inno-agent/backend/review-api

go 1.26.0

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/inno-agent/inno-agent/backend/metrics v0.0.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/joho/godotenv v1.5.1
	go.uber.org/zap v1.28.0
)

replace github.com/inno-agent/inno-agent/backend/metrics => ../metrics

require (
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/text v0.31.0 // indirect
)
