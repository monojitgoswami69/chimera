#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${CHIMERA_INSTALL_DIR:-$HOME/.local/bin}"
REPO_URL="https://github.com/monojitgoswami69/chimera.git"

err() { echo "error: $*" >&2; exit 1; }

command -v go  >/dev/null || err "Go 1.21+ required. https://go.dev/doc/install"
command -v git >/dev/null || err "git required."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "→ cloning chimera into $tmp..."
git -C "$tmp" clone --depth=1 "$REPO_URL"

echo "→ building..."
( cd "$tmp/chimera" && go build -ldflags "-s -w" -o chimera . )

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/chimera/chimera" "$INSTALL_DIR/chimera"

echo
echo "✓ installed → $INSTALL_DIR/chimera"
case ":${PATH}:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "  add '$INSTALL_DIR' to your PATH:"
     echo "    export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
echo
echo "next: chimera setup    (or:  chimera init --no-agent https://github.com/<owner>/<repo>)"
