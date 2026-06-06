SELECT 'CREATE DATABASE inno_auth'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'inno_auth')\gexec
