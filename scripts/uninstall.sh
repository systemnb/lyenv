#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${1:-lyenv}"

echo "Uninstalling $APP_NAME..."

PATHS=(
  "/usr/local/bin/$APP_NAME"
  "${HOME}/.local/bin/$APP_NAME"
  "${HOME}/bin/$APP_NAME"
)

removed=false
for p in "${PATHS[@]}"; do
  if [[ -f "$p" ]]; then
    if [[ "$p" == /usr/local/bin/* ]]; then
      if command -v sudo >/dev/null 2>&1; then
        sudo rm -f "$p"
        echo "Removed: $p"
        removed=true
      else
        echo "Cannot remove $p without sudo."
      fi
    else
      rm -f "$p"
      echo "Removed: $p"
      removed=true
    fi
  fi
done

if ! $removed; then
  echo "No installed binary found."
fi

if [[ -f "$HOME/.${SHELL_NAME}rc" ]]; then
      sed -i '/###lyenv###/,/###lyenv###/d' "$HOME/.${SHELL_NAME}rc"
      echo "Removed from $HOME/.${SHELL_NAME}rc"
fi


echo "Done."
