#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"
DIST_DIR="$WEB_DIR/dist"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

cp "$WEB_DIR/index.html" "$DIST_DIR/index.html"
cp "$WEB_DIR/styles.css" "$DIST_DIR/styles.css"
cp "$WEB_DIR/app.js" "$DIST_DIR/app.js"
cp "$WEB_DIR/worker.js" "$DIST_DIR/worker.js"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$DIST_DIR/wasm_exec.js"

GOOS=js GOARCH=wasm go build -o "$DIST_DIR/fit_analyzer.wasm" ./cmd/fit_wasm
