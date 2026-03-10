#!/bin/bash
# init-db.sh – Run on ClickHouse first start.
# Creates the seebom database and runs all migrations.
set -e

echo "⏳ Creating seebom database..."
clickhouse-client --query "CREATE DATABASE IF NOT EXISTS seebom"

echo "⏳ Running migrations..."
for f in /docker-entrypoint-initdb.d/migrations/*.sql; do
  echo "  → $f"
  clickhouse-client --database=seebom --multiquery < "$f"
done

echo "✅ Database initialized."

