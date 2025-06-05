#!/bin/sh
set -e
trap 'kill $(jobs -p) 2>/dev/null' EXIT
air &
cd web && npm run dev -- --host 0.0.0.0
