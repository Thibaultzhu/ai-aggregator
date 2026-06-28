#!/usr/bin/env bash
# Source this file from the project root:
#   source scripts/dev-env.sh

if [ -n "${BASH_SOURCE[0]:-}" ]; then
  SCRIPT_PATH="${BASH_SOURCE[0]}"
elif [ -n "${ZSH_VERSION:-}" ]; then
  SCRIPT_PATH="${(%):-%N}"
else
  SCRIPT_PATH="$0"
fi

ROOT_DIR="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"

export PATH="$ROOT_DIR/.local/go/bin:$ROOT_DIR/.local/bin:$PATH"
export DOCKER_CONFIG="$ROOT_DIR/.docker-config"
export BUILDX_CONFIG="$ROOT_DIR/.docker-buildx"
export GOCACHE="$ROOT_DIR/.local/go-cache"
export GOMODCACHE="$ROOT_DIR/.local/go-mod-cache"

mkdir -p "$GOCACHE" "$GOMODCACHE"

echo "AI Aggregator dev environment loaded"
echo "PATH includes: $ROOT_DIR/.local/go/bin and $ROOT_DIR/.local/bin"
echo "DOCKER_CONFIG=$DOCKER_CONFIG"
echo "BUILDX_CONFIG=$BUILDX_CONFIG"
echo "GOCACHE=$GOCACHE"
echo "GOMODCACHE=$GOMODCACHE"
