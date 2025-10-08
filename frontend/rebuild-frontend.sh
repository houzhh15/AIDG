#!/usr/bin/env bash
set -euo pipefail
# Cross-platform (Unix) front-end rebuild helper.
# Usage: ./rebuild-frontend.sh [--install-only|--skip-install]
# 1. Detect package manager (pnpm>yarn>npm) else default npm
# 2. Clean previous build dist & ts buildinfo
# 3. Install deps (unless --skip-install)
# 4. Run build
# 5. Print dist summary

cd "$(dirname "$0")"

PM=""
if command -v pnpm >/dev/null 2>&1; then PM=pnpm
elif command -v yarn >/dev/null 2>&1; then PM=yarn
else PM=npm
fi

SKIP_INSTALL=false
INSTALL_ONLY=false
for arg in "$@"; do
  case "$arg" in
    --skip-install) SKIP_INSTALL=true ;;
    --install-only) INSTALL_ONLY=true ;;
  esac
done

echo "[frontend] package manager: $PM"

if [ "$SKIP_INSTALL" = false ]; then
  echo "[frontend] installing dependencies..."
  if [ "$PM" = yarn ]; then
    yarn install --frozen-lockfile || yarn install
  elif [ "$PM" = pnpm ]; then
    pnpm install --frozen-lockfile || pnpm install
  else
    npm install
  fi
fi

if [ "$INSTALL_ONLY" = true ]; then
  echo "[frontend] install-only mode complete"
  exit 0
fi

echo "[frontend] cleaning previous build artifacts..."
rm -rf dist
rm -f tsconfig.tsbuildinfo

echo "[frontend] building..."
if [ "$PM" = yarn ]; then
  yarn build
elif [ "$PM" = pnpm ]; then
  pnpm build
else
  npm run build
fi

echo "[frontend] build complete. Dist contents:"
find dist -maxdepth 2 -type f -print | head -n 30
