#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export PATH="$ROOT_DIR/.local/go/bin:$ROOT_DIR/.local/bin:$PATH"
export DOCKER_CONFIG="$ROOT_DIR/.docker-config"
export BUILDX_CONFIG="$ROOT_DIR/.docker-buildx"
export GOCACHE="$ROOT_DIR/.local/go-cache"
export GOMODCACHE="$ROOT_DIR/.local/go-mod-cache"

mkdir -p "$GOCACHE" "$GOMODCACHE"

echo "== Toolchain =="
go version
node --version
npm --version
docker --version
docker compose version
helm version --short

echo
echo "== Services =="
docker compose ps

echo
echo "== App health =="
curl -fsS http://localhost:8081/health
echo
curl -fsS http://localhost:5175 >/dev/null
echo "frontend ok"

echo
echo "== Helm render =="
helm template ai-aggregator deploy/helm/ai-aggregator \
  --set backend.secrets.DATABASE_URL=postgres://aag:aag_dev_pass@postgres:5432/aggregator?sslmode=disable \
  --set backend.secrets.REDIS_URL=redis://redis:6379/0 \
  --set backend.secrets.JWT_SECRET=dev-secret-change-me >/tmp/aag-helm-render.yaml
echo "helm template ok: /tmp/aag-helm-render.yaml"
