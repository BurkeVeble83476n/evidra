#!/usr/bin/env bash
# Local build and push of Docker images to GHCR.
# Skips CI entirely — use when you've already verified locally.
#
# Usage:
#   ./scripts/docker-push.sh              # build + push all images as :latest
#   ./scripts/docker-push.sh v0.5.15      # build + push with version tags
#   ./scripts/docker-push.sh v0.5.15 api  # build + push only evidra-api
set -euo pipefail

REGISTRY="ghcr.io/vitas"
VERSION="${1:-latest}"
TARGET="${2:-all}"

# Image matrix — same as release.yml
declare -A IMAGES=(
  [evidra-api]="Dockerfile.api"
  [evidra-mcp]="Dockerfile"
  [evidra]="Dockerfile.cli"
  [evidra-mcp-hosted]="Dockerfile.hosted"
)

cd "$(git rev-parse --show-toplevel)"

# Login check
if ! docker info 2>/dev/null | grep -q "Username"; then
  echo "Logging in to GHCR..."
  echo "${GHCR_TOKEN:-}" | docker login ghcr.io -u "${GHCR_USERNAME:-vitas}" --password-stdin
fi

build_and_push() {
  local name="$1"
  local dockerfile="$2"
  local tag="${REGISTRY}/${name}:${VERSION}"
  local latest="${REGISTRY}/${name}:latest"

  echo "=== Building ${name} (${dockerfile}) ==="
  docker build -f "${dockerfile}" -t "${tag}" .

  # Smoke test
  echo "  Smoke test: ${tag} --version"
  docker run --rm "${tag}" --version || true

  echo "  Pushing ${tag}"
  docker push "${tag}"

  if [ "${VERSION}" != "latest" ]; then
    docker tag "${tag}" "${latest}"
    echo "  Pushing ${latest}"
    docker push "${latest}"
  fi

  echo "  Done: ${name}"
  echo
}

if [ "${TARGET}" = "all" ]; then
  for name in "${!IMAGES[@]}"; do
    build_and_push "${name}" "${IMAGES[$name]}"
  done
else
  dockerfile="${IMAGES[$TARGET]:-}"
  if [ -z "${dockerfile}" ]; then
    echo "Unknown target: ${TARGET}" >&2
    echo "Available: ${!IMAGES[*]}" >&2
    exit 1
  fi
  build_and_push "${TARGET}" "${dockerfile}"
fi

echo "All done. Version: ${VERSION}"
