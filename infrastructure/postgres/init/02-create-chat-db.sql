SELECT 'CREATE DATABASE llm_chat'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'llm_chat')\gexec
