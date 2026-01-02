#!/usr/bin/env bash
set -euo pipefail

BIN_SRC="${1:-}"
APP_NAME="${2:-lyenv}"

if [[ -z "$BIN_SRC" || ! -f "$BIN_SRC" ]]; then
  echo "Error: binary not found: $BIN_SRC"
  exit 2
fi

echo "Installing $APP_NAME..."

# Prefer per-user bin
USER_BIN="${HOME}/.local/bin"
ALT_BIN="${HOME}/bin"
SYS_BIN="/usr/local/bin"

TARGET=""

# Ensure user bin exists or create
if [[ -d "$USER_BIN" || ( -w "$(dirname "$USER_BIN")" && ! -e "$USER_BIN" ) ]]; then
  mkdir -p "$USER_BIN"
  TARGET="$USER_BIN/$APP_NAME"
elif [[ -d "$ALT_BIN" || ( -w "$(dirname "$ALT_BIN")" && ! -e "$ALT_BIN" ) ]]; then
  mkdir -p "$ALT_BIN"
  TARGET="$ALT_BIN/$APP_NAME"
else
  # Try system-wide when sudo is available
  if command -v sudo >/dev/null 2>&1; then
    echo "User bin not writable; attempting system install to $SYS_BIN (sudo required)."
    sudo install -m 0755 "$BIN_SRC" "$SYS_BIN/$APP_NAME"
    echo "Installed to: $SYS_BIN/$APP_NAME"
    echo "Done."
    exit 0
  else
    echo "No writable user bin and sudo not available."
    echo "Please create ~/bin or ~/.local/bin and add it to PATH."
    exit 1
  fi
fi

# Copy to target
install -m 0755 "$BIN_SRC" "$TARGET"
echo "Installed to: $TARGET"

# Ensure PATH contains the target dir
TARGET_DIR="$(dirname "$TARGET")"
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$TARGET_DIR"; then
  SHELL_NAME="$(basename "${SHELL:-sh}")"
  PROFILE_FILES=(~/.bashrc ~/.zshrc ~/.profile)
  echo "NOTICE: $TARGET_DIR is not in PATH."
  echo "To add it, append the following to your shell profile:"
  echo "  export PATH=\"$TARGET_DIR:\$PATH\""
  for f in "${PROFILE_FILES[@]}"; do
    if [[ -f "$f" ]]; then
      echo "You may add it to: $f"
    fi
  done
  read -rp "Would you like to add it to your $SHELL_NAME profile? (y/N) " yn
  case "$yn" in
    [Yy]*)
      echo "###lyenv###" >> "$HOME/.${SHELL_NAME}rc"
      echo "export PATH=\"$TARGET_DIR:\$PATH\"" >> "$HOME/.${SHELL_NAME}rc"
      echo "###lyenv###" >> "$HOME/.${SHELL_NAME}rc"
      echo "Added to $HOME/.${SHELL_NAME}rc"
      ;;
    *)
      echo "Not adding to profile."
      ;;
  esac
fi

echo "Done."
