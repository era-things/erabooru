#!/bin/sh
set -eu

lock_hash="$(sha256sum package-lock.json | cut -d ' ' -f 1)"
marker="node_modules/.package-lock.sha256"

if [ ! -f "$marker" ] || [ "$(cat "$marker")" != "$lock_hash" ]; then
	npm ci --no-audit --no-fund
	printf '%s' "$lock_hash" > "$marker"
fi

exec npm run dev -- --host 0.0.0.0
