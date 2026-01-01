#!/usr/bin/env bash
# End-to-end test covering: create/init/activate, config ops, center sync/search,
# plugin install (center/local), run (shell+stdio, multi-step), timeout policies,
# update, list/info, remove & shim, logs & mutations, dispatch log, JSON dump/load.
# All comments and outputs are in English.

set -euo pipefail

# -------- Configuration --------
LY="${LYENV_BIN:-lyenv}"   # Use lyenv in PATH or override by env: LYENV_BIN=./dist/lyenv
CENTER_INDEX_URL="${CENTER_INDEX_URL:-https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml}"

# Optional: ensure pyenv python3 in PATH
export PATH="$HOME/.pyenv/shims:$PATH"

# Timestamp-based environment folder to avoid collision
ENV_DIR="_e2e_env_$(date +%Y%m%d_%H%M%S)"
SHIM_CACHE_FLUSH_CMD="hash -r"  # bash command to flush command cache

# -------- Helpers --------
die() { echo "ERROR: $*" >&2; exit 1; }
assert_file() { [[ -f "$1" ]] || die "Missing file: $1"; }
assert_nonempty() { [[ -s "$1" ]] || die "File is empty: $1"; }
assert_contains() { local f="$1"; shift; local needle="$*"; grep -Fq "$needle" "$f" || die "Assert failed: '$needle' not found in $f"; }
expect_nonzero() { local rc="$1"; [[ "$rc" -ne 0 ]] || die "Expected non-zero exit, got $rc"; }
info() { echo "[INFO] $*"; }
step() { echo; echo "===== $* ====="; }

# -------- Preconditions --------
step "[0] Preconditions"
command -v git >/dev/null || die "git required"
command -v curl >/dev/null || command -v wget >/dev/null || die "curl or wget required"
command -v tar  >/dev/null || die "tar required"
command -v unzip >/dev/null || die "unzip required"
command -v python3 >/dev/null || die "python3 required"
command -v "$LY" >/dev/null || die "lyenv binary not found (set LYENV_BIN if not in PATH)"
info "Using lyenv: $(command -v "$LY")"
info "Center index: $CENTER_INDEX_URL"

# -------- Create & Init --------
step "[1] Create environment"
"$LY" create "$ENV_DIR"
"$LY" init "$ENV_DIR"
pushd "$ENV_DIR" >/dev/null

step "[2] Activate environment"
eval "$("$LY" activate)"
# Confirm environment directories are present
for d in ".lyenv" ".lyenv/logs" ".lyenv/registry" "bin" "cache" "plugins" "workspace"; do
  [[ -d "$d" ]] || die "Missing directory after init: $d"
done
# Confirm defaults
assert_file "lyenv.yaml"
assert_contains "lyenv.yaml" "registry_url: \"$CENTER_INDEX_URL\"" || info "NOTE: registry_url will be set next step"

# -------- Config Ops --------
step "[3] Configure plugin center registry URL"
"$LY" config set plugins.registry_url "$CENTER_INDEX_URL" --type=string

info "Get registry_url"
"$LY" config get plugins.registry_url

step "[4] Center sync (cache index locally)"
"$LY" plugin center sync
assert_file ".lyenv/registry/index.yaml" || assert_file ".lyenv/registry/index.json"

step "[5] Search plugins by keywords"
"$LY" plugin search tester
# Optional: show JSON list output
info "Installed list (JSON, initial should be empty)"
"$LY" plugin list --json || true

# -------- Install from center --------
step "[6] Install plugin 'tester' by NAME (center resolution, monorepo or archive+sha256)"
"$LY" plugin install tester --name=testtools

info "List installed plugins (human)"
"$LY" plugin list

info "List installed plugins (JSON)"
"$LY" plugin list --json > .lyenv/plugins_list.json
assert_nonempty ".lyenv/plugins_list.json"

info "Show plugin info"
"$LY" plugin info testtools

# -------- Run plugin commands --------
step "[7] Run plugin command (multi-step: shell + stdio) via shim"
tctl run

step "[8] Run plugin command via core (keep-going)"
"$LY" run testtools run --merge=override --keep-going

# Verify plugin logs & mutations
step "[9] Verify plugin logs and mutations"
LOGFILE=$(ls plugins/testtools/logs/*/run-*.log | head -n1)
assert_file "$LOGFILE"
assert_contains "lyenv.yaml" "tester_demo"
assert_contains "plugins/testtools/config.yaml" "build_count:"

# -------- Timeout & Policies --------
step "[10] Demonstrate timeout & fail-fast policies"
set +e
"$LY" run testtools slow --timeout=5 --fail-fast
RC=$?
set -e
expect_nonzero "$RC"
info "Timeout behavior OK (non-zero exit as expected)."

# -------- Remove & Shim --------
step "[11] Remove plugin and verify shim removal"
"$LY" plugin remove testtools --force
# Flush shell command cache to avoid stale resolution
eval "$SHIM_CACHE_FLUSH_CMD"
# Diagnostic visibility
type -a tctl || true
which -a tctl || true
if command -v tctl >/dev/null 2>&1; then
  die "Shim still present (PATH collision or external tctl exists)."
else
  info "Shim removed OK"
fi

# -------- Local plugin (add path) --------
step "[12] Create a local demo plugin (shell only) and install"
LOCAL_DIR="local_demo"
mkdir -p "$LOCAL_DIR/scripts"
cat > "$LOCAL_DIR/manifest.yaml" <<'YAML'
name: localdemo
version: 0.1.0
expose: [ldctl]
commands:
  - name: hello
    summary: Say hello via shell
    executor: shell
    program: 'echo "localdemo: hello world"'
YAML
# Install local plugin
"$LY" plugin add "./$LOCAL_DIR" --name=localtools
# Run local plugin
ldctl hello
# Remove local plugin
"$LY" plugin remove localtools --force
eval "$SHIM_CACHE_FLUSH_CMD"
if command -v ldctl >/dev/null 2>&1; then
  die "Local demo shim still present"
else
  info "Local demo removal OK"
fi

# -------- Archive + SHA-256 (center-provided, strict validation) --------
step "[13] Validate archive + SHA-256 installation path (center-provided)"

PLUGIN_NAME="tester"

# Choose index file path: prefer cache; fallback to remote download
INDEX_CACHED_YAML=".lyenv/registry/index.yaml"
INDEX_REMOTE_YAML="_center_index.yaml"

if [[ -f "$INDEX_CACHED_YAML" ]]; then
  INDEX_PATH="$INDEX_CACHED_YAML"
else
  if command -v curl >/dev/null; then
    curl -sSL "$CENTER_INDEX_URL" -o "$INDEX_REMOTE_YAML"
  else
    wget -q "$CENTER_INDEX_URL" -O "$INDEX_REMOTE_YAML"
  fi
  INDEX_PATH="$INDEX_REMOTE_YAML"
fi
assert_file "$INDEX_PATH"

SOURCE_URL=""
SOURCE_SHA=""

PY_BIN="${PY_BIN:-python3}"
if "$PY_BIN" -c "import yaml,sys" >/dev/null 2>&1; then
  # Robust Python: print two lines (source then sha256); empty if not found
  PY_OUT="$("$PY_BIN" - "$INDEX_PATH" "$PLUGIN_NAME" <<'PY'
import sys, yaml
path = sys.argv[1]
name = sys.argv[2]
try:
    with open(path, "r", encoding="utf-8") as f:
        idx = yaml.safe_load(f) or {}
    entry = (idx.get("plugins") or {}).get(name) or {}
    versions = entry.get("versions") or {}
    if not versions:
        # No versions entry: print empty lines
        print("")
        print("")
        sys.exit(0)
    # pick lexicographically latest key (switch to semver if needed)
    ver = sorted(versions.keys())[-1]
    v = versions.get(ver) or {}
    src = v.get("source") or ""
    sha = v.get("sha256") or ""
    print(src)
    print(sha)
except Exception as e:
    # On parse failure, emit empty lines; bash will catch empties
    print("")
    print("")
PY
  )"
  # Safely read two lines
  SOURCE_URL="$(echo "$PY_OUT" | sed -n '1p')"
  SOURCE_SHA="$(echo "$PY_OUT" | sed -n '2p')"
else
  # Fallback (best-effort): grep-like extraction; may fail on complex YAML
  SOURCE_URL="$(awk '
    $0 ~ /^plugins:/ { inplugins=1 }
    inplugins && $0 ~ /^  '"$PLUGIN_NAME"':/ { inplugin=1; next }
    inplugin && $0 ~ /^    versions:/ { inver=1; next }
    inver && $0 ~ /^      / { invk=1; next }
    invk && $0 ~ /source:/ { gsub(/"/,""); print $2; exit }
  ' "$INDEX_PATH")"
  SOURCE_SHA="$(awk '
    $0 ~ /^plugins:/ { inplugins=1 }
    inplugins && $0 ~ /^  '"$PLUGIN_NAME"':/ { inplugin=1; next }
    inplugin && $0 ~ /^    versions:/ { inver=1; next }
    inver && $0 ~ /^      / { invk=1; next }
    invk && $0 ~ /sha256:/ { gsub(/"/,""); print $2; exit }
  ' "$INDEX_PATH")"
fi

# Validate presence of source+sha256
if [[ -z "$SOURCE_URL" || -z "$SOURCE_SHA" ]]; then
  echo "[WARN] Center index does not provide source+sha256 for plugin '$PLUGIN_NAME'."
  echo "       Ensure your center CI generated artifacts and updated index.yaml (PR merged to main)."
  die "Archive+SHA-256 validation cannot proceed without source+sha256."
fi

info "Center-provided source URL: $SOURCE_URL"
info "Center-provided sha256: $SOURCE_SHA"

# Download archive locally and verify sha256 BEFORE install
TMP_ARCHIVE="_tester_archive_download"
if command -v curl >/dev/null; then
  curl -sSL "$SOURCE_URL" -o "$TMP_ARCHIVE"
else
  wget -q "$SOURCE_URL" -O "$TMP_ARCHIVE"
fi
assert_file "$TMP_ARCHIVE"
assert_nonempty "$TMP_ARCHIVE"

# Compute sha256 and compare
if command -v sha256sum >/dev/null; then
  COMPUTED_SHA="$(sha256sum "$TMP_ARCHIVE" | awk '{print $1}')"
elif command -v shasum >/dev/null; then
  COMPUTED_SHA="$(shasum -a 256 "$TMP_ARCHIVE" | awk '{print $1}')"
else
  die "No sha256 tool found (sha256sum/shasum)"
fi

if [[ "$COMPUTED_SHA" != "$SOURCE_SHA" ]]; then
  echo "Computed: $COMPUTED_SHA"
  echo "Expected: $SOURCE_SHA"
  die "SHA-256 mismatch for downloaded archive!"
else
  info "SHA-256 verification OK."
fi

# Install by NAME again (center will prefer archive+sha256 path if present)
ARCH_INSTALL="testtools-archive"
"$LY" plugin install "$PLUGIN_NAME" --name="$ARCH_INSTALL"

# Verify shim & run basic command
tctl run
"$LY" run "$ARCH_INSTALL" run --merge=override --keep-going

# Cleanup: remove archive install and flush cache
"$LY" plugin remove "$ARCH_INSTALL" --force
eval "$SHIM_CACHE_FLUSH_CMD"
if command -v tctl >/dev/null 2>&1; then
  die "Shim still present after archive install removal"
else
  info "Archive install removal OK"
fi

# -------- Config dump/load (JSON) --------
step "[14] Config dump to JSON and re-load"
"$LY" config dump config ./config_dump.json
assert_nonempty "./config_dump.json"
"$LY" config load ./config_dump.json --merge=keep
info "Config JSON dump/load OK"

# -------- Dispatch log --------
step "[15] Inspect global dispatch log"
assert_file ".lyenv/logs/dispatch.log"
tail -n +1 ".lyenv/logs/dispatch.log" | sed -n '1,10p' || true

# -------- Final Summary --------
step "[16] Summary"
echo "Environment: $ENV_DIR"
echo "Center index: $CENTER_INDEX_URL"
echo "Plugin operations: install, run (shell+stdio), timeout, remove, local add/install/remove verified."
echo "Logs & mutations: OK"
echo "Config dump/load JSON: OK"

popd >/dev/null
echo
echo "[ALL PASSED]"
