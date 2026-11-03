#!/usr/bin/env bash
# Regenerate the report published on GitHub Pages.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p docs
go run . -f examples/pods.json -o html > docs/index.html
echo "wrote docs/index.html"
