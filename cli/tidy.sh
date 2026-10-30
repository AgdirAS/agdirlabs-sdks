#!/bin/sh
# Runs `go mod tidy` for the CLI module while the SDK tag go/vX.Y.Z does not exist yet:
# temporarily points the SDK at ../go with a replace directive, then removes it again.
set -eu
cd "$(dirname "$0")"
go mod edit -require github.com/AgdirAS/agdirlabs-sdks/go@v0.1.0 -replace github.com/AgdirAS/agdirlabs-sdks/go=../go
trap 'go mod edit -dropreplace github.com/AgdirAS/agdirlabs-sdks/go' EXIT
GOWORK=off go mod tidy "$@"
