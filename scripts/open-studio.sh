#!/usr/bin/env bash
# Launch Blog Studio (local-only writing UI) for nacfson_blog.
set -euo pipefail

SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
ROOT="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"
ADDR="${STUDIO_ADDR:-127.0.0.1:4321}"
BRANCH="${STUDIO_PUBLISH_BRANCH:-main}"

cd "$ROOT"
[ -x bin/blog-studio ] || make studio

(sleep 1; xdg-open "http://${ADDR}" >/dev/null 2>&1 || true) &
exec ./bin/blog-studio --repo . --addr "$ADDR" --publish-branch "$BRANCH"
