#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
BASE=/tmp/ohmycore-smoke
rm -rf "$BASE"
mkdir -p "$BASE"
go build -o /tmp/ohmycore ./cmd/ohmycore
ID=$(/tmp/ohmycore --base-dir "$BASE" instance create --kernel xray --config ./testdata/xray.json)
/tmp/ohmycore --base-dir "$BASE" instance start "$ID"
echo "started $ID"
/tmp/ohmycore --base-dir "$BASE" instance stop "$ID"
echo "smoke ok"