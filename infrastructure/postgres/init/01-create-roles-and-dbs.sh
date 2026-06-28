#!/bin/sh
# Runs once on first start (empty postgres-data volume).
# Creates one dedicated login role per service; each DB is owned by its role,
# so no service runs as the postgres superuser.
set -e

psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<-EOSQL
	CREATE ROLE authentik LOGIN PASSWORD '${AUTHENTIK_PG_PASSWORD}';
	CREATE ROLE identity  LOGIN PASSWORD '${IDENTITY_PG_PASSWORD}';
	CREATE ROLE chat      LOGIN PASSWORD '${CHAT_PG_PASSWORD}';
	CREATE ROLE review    LOGIN PASSWORD '${REVIEW_PG_PASSWORD}';

	ALTER DATABASE "${POSTGRES_DB}" OWNER TO authentik;
	CREATE DATABASE inno_auth   OWNER identity;
	CREATE DATABASE llm_chat    OWNER chat;
	CREATE DATABASE inno_review OWNER review;
EOSQL
