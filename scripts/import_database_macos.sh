#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATION_ROOT="${PROJECT_ROOT}/migration"
DUMP_FILE="${MIGRATION_ROOT}/database/testcenter_business_20260803.sql"
COUNTS_FILE="${MIGRATION_ROOT}/database/table_row_counts.csv"
MAC_CONFIG="${MIGRATION_ROOT}/config/database_config.mac.json"
PASSWORD_FILE="${MIGRATION_ROOT}/secrets/mac_mysql_app_password.txt"

DB_NAME="testcenter"
DB_USER="testcenter"
DB_HOST="127.0.0.1"
DB_PORT="3306"
ROOT_USER="${MYSQL_ROOT_USER:-root}"
ROOT_HOST="${MYSQL_ROOT_HOST:-localhost}"
ROOT_PORT="${MYSQL_ROOT_PORT:-3306}"

fail() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || fail "This script must run on macOS."
command -v mysql >/dev/null 2>&1 || fail "mysql client is missing. Run: brew install mysql"
[[ -s "$DUMP_FILE" ]] || fail "Database dump is missing: $DUMP_FILE"
[[ -s "$COUNTS_FILE" ]] || fail "Row-count manifest is missing: $COUNTS_FILE"
[[ -s "$MAC_CONFIG" ]] || fail "Mac database config is missing: $MAC_CONFIG"
[[ -s "$PASSWORD_FILE" ]] || fail "Mac database password file is missing: $PASSWORD_FILE"

APP_PASSWORD="$(tr -d '\r\n' < "$PASSWORD_FILE")"
[[ "$APP_PASSWORD" =~ ^[A-Za-z0-9]+$ ]] || fail "Generated database password has an invalid format."

if [[ "${MIGRATION_CONFIRM:-}" != "YES" ]]; then
  printf 'This will replace the local MySQL database `%s` on this Mac.\n' "$DB_NAME"
  read -r -p 'Type YES to continue: ' answer
  [[ "$answer" == "YES" ]] || fail "Import cancelled."
fi

if [[ -z "${MYSQL_ROOT_PASSWORD+x}" ]]; then
  read -r -s -p "MySQL ${ROOT_USER} password (leave empty if none): " MYSQL_ROOT_PASSWORD
  printf '\n'
fi

ROOT_ARGS=(--protocol=TCP --host="$ROOT_HOST" --port="$ROOT_PORT" --user="$ROOT_USER" --default-character-set=utf8mb4)

root_mysql() {
  if [[ -n "${MYSQL_ROOT_PASSWORD:-}" ]]; then
    MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql "${ROOT_ARGS[@]}" "$@"
  else
    mysql "${ROOT_ARGS[@]}" "$@"
  fi
}

printf 'Creating clean database `%s`...\n' "$DB_NAME"
root_mysql --execute="
DROP DATABASE IF EXISTS \`${DB_NAME}\`;
CREATE DATABASE \`${DB_NAME}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '${DB_USER}'@'localhost' IDENTIFIED BY '${APP_PASSWORD}';
ALTER USER '${DB_USER}'@'localhost' IDENTIFIED BY '${APP_PASSWORD}';
CREATE USER IF NOT EXISTS '${DB_USER}'@'127.0.0.1' IDENTIFIED BY '${APP_PASSWORD}';
ALTER USER '${DB_USER}'@'127.0.0.1' IDENTIFIED BY '${APP_PASSWORD}';
GRANT ALL PRIVILEGES ON \`${DB_NAME}\`.* TO '${DB_USER}'@'localhost';
GRANT ALL PRIVILEGES ON \`${DB_NAME}\`.* TO '${DB_USER}'@'127.0.0.1';
FLUSH PRIVILEGES;"

printf 'Importing database snapshot...\n'
MYSQL_PWD="$APP_PASSWORD" mysql \
  --protocol=TCP \
  --host="$DB_HOST" \
  --port="$DB_PORT" \
  --user="$DB_USER" \
  --default-character-set=utf8mb4 \
  "$DB_NAME" < "$DUMP_FILE"

printf 'Installing Mac database configuration...\n'
cp "$MAC_CONFIG" "${PROJECT_ROOT}/server/data/database_config.json"
chmod 600 "${PROJECT_ROOT}/server/data/database_config.json"

printf 'Verifying table row counts...\n'
failures=0
while IFS=, read -r raw_table raw_expected; do
  table="$(printf '%s' "$raw_table" | tr -d '\r\"')"
  expected="$(printf '%s' "$raw_expected" | tr -d '\r\"')"
  [[ "$expected" =~ ^[0-9]+$ ]] || continue
  actual="$(MYSQL_PWD="$APP_PASSWORD" mysql --protocol=TCP --host="$DB_HOST" --port="$DB_PORT" --user="$DB_USER" --database="$DB_NAME" --batch --skip-column-names --execute="SELECT COUNT(*) FROM \`${table}\`")"
  if [[ "$actual" != "$expected" ]]; then
    printf '  FAIL %-32s expected=%s actual=%s\n' "$table" "$expected" "$actual"
    failures=$((failures + 1))
  else
    printf '  OK   %-32s rows=%s\n' "$table" "$actual"
  fi
done < "$COUNTS_FILE"

[[ "$failures" -eq 0 ]] || fail "$failures table(s) failed row-count verification."

printf '\nDatabase migration completed successfully.\n'
printf 'Database: %s\nConfig:   %s\n' "$DB_NAME" "${PROJECT_ROOT}/server/data/database_config.json"
