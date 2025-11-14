#!/bin/bash
# Build bd-tui binary

set -e

cd "$(dirname "$0")"

go build -o bd-tui ./cmd/bd-tui

echo "Built: ./bd-tui"
