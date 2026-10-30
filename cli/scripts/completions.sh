#!/bin/sh
# Generates shell completion scripts into completions/ (run by GoReleaser before building).
set -eu
cd "$(dirname "$0")/.."
rm -rf completions
mkdir -p completions
for shell in bash zsh fish powershell; do
	go run ./cmd/agdir-email completion "$shell" >"completions/agdir-email.$shell"
done
mv completions/agdir-email.powershell completions/agdir-email.ps1
