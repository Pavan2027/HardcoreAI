#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INTEGRATION_DIR="$ROOT_DIR/integration"

mkdir -p "$ROOT_DIR/data" "$INTEGRATION_DIR/uploads" "$INTEGRATION_DIR/logs"

if [ ! -f "$ROOT_DIR/.env.example" ]; then
  cp "$INTEGRATION_DIR/backend/.env.example" "$ROOT_DIR/.env.example"
fi

echo "Building rag-cli into integration/rag-cli"
cd "$ROOT_DIR"
go run build.go

cat <<'EOF'
Setup complete.

Next steps:
1. Copy integration/backend/.env.example to your backend .env
2. Set RAG_CLI_PATH, RAG_DB_PATH, RAG_DATA_DIR, and UPLOAD_DIR
3. Put PDFs into the chosen RAG_DATA_DIR
4. Run: integration/rag-cli ingest --db <db> --dir <data-dir>
EOF
