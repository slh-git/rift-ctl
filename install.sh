#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "Building and installing riftctl..."
go install ./cmd/riftctl

dest="$(go env GOBIN)"
if [[ -z "$dest" ]]; then
  dest="$(go env GOPATH)/bin"
fi

echo "Installed: ${dest}/riftctl"
riftctl --help >/dev/null
echo "OK — run: riftctl inspect teemo"
