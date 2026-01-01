# lyenv — Directory-based Isolated Environment Manager

> This README is provided in [English](README.md) and [中文](README_zh.md) (Chinese).  
> 语言切换：下方包含 [English](README.md) 和 [中文](README_zh.md)，内容一致。  
> License: See `LICENSE` at the repository root.

---

## English Version

### 1. What is lyenv? (Project Meaning and Goals)

**lyenv** is a simple, robust, and language-agnostic environment manager based on a **directory layout**. It lets you:

- Create and activate isolated workspaces (`bin/`, `plugins/`, `workspace/`, `.lyenv/`).
- Install and run **plugins** implemented in any language (Python, Node.js, C/C++, etc.), via:
  - **shell executor** (run command lines and capture logs),
  - **stdio executor** (exchange JSON over stdin/stdout, enabling structured results and configuration mutations).
- Merge **structured configuration** from plugins (`mutations`) into:
  - global config (`lyenv.yaml`),
  - plugin-local config (`plugins/<INSTALL_NAME>/config.yaml` or `.json` by extension).
- Record per-command **JSON Lines logs** and a global **dispatch log** for observability.
- Resolve plugins by **name** from a **Plugin Center (Monorepo)** with subdirs, optionally using **archive + SHA‑256** integrity verification.

**Why this matters**:

- **Language-agnostic**: Any tool that can run in shell or read/write JSON over stdio is supported.
- **Reproducible & Traceable**: Directory-based design, JSON logs, and dispatch records simplify auditing and debugging.
- **Structured orchestration**: `stdio` enables returning structured results and safe config mutations, avoiding brittle text scraping.
- **Scalable distribution**: Plugin Center can host many plugins as **subdirectories** or **prebuilt archives** with SHA‑256 verification.

---

### 2. Quick Start

#### 2.1 Build

```bash
make build
# or
go build -o ./dist/lyenv ./cmd/lyenv
```

#### 2.2 Create, Init, Activate

```bash
./dist/lyenv create ./my-env
./dist/lyenv init ./my-env
cd ./my-env
eval "$("./../dist/lyenv" activate)"
```

After create:

Directory structure:
```
.lyenv/, .lyenv/logs, .lyenv/registry, bin/, cache/, plugins/, workspace/.
```

`lyenv.yaml` defaults (center URL included):

```yaml
env:
  name: "default"
  platform: "auto"
path:
  bin: "./bin"
  cache: "./cache"
  workspace: "./workspace"
plugins:
  installed: []
  registry_url: "https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml"
  registry_format: "yaml"
  default_version_strategy: "latest"
config:
  use_container: false
  pkg_manager: "auto"
  network:
    proxy_url: ""   # set to e.g. http://127.0.0.1:7890 if needed
```

Registry file initialized: `.lyenv/registry/installed.yaml`:
```yaml
plugins: []
```

---

### 3. Commands: Tutorials and Usage

All program outputs are in English. Below are the major commands.

#### 3.1 Environment

```bash
lyenv create <DIR>
# Create a new environment with default folders and config

lyenv init <DIR>
# Verify/repair an existing environment (idempotent)

lyenv activate
# Print shell snippet; typically: eval "$(lyenv activate)"
# The snippet is robust in non-interactive shells (checks PS1 existence)
```

#### 3.2 Config Management

```bash
lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]
# Set a value under lyenv.yaml (dot path with type enforcement)

lyenv config get <KEY>
# Read a value by dot path

lyenv config dump [<KEY>] <FILE>
# Dump entire config or a specific key to FILE (YAML/JSON by extension)

lyenv config load <FILE> [--merge=override|append|keep]
# Load YAML or JSON overlay into lyenv.yaml with merge strategy

lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=...] [--merge=...] [--input=1]
# Import from JSON file (dot path) into lyenv.yaml

lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=...] [--merge=...] [--input=1]
# Import from YAML file (dot path) into lyenv.yaml
```

#### 3.3 Plugin Center and Search

```bash
lyenv plugin center sync
# Cache center index into .lyenv/registry/index.yaml or .json

lyenv plugin search <KEYWORDS...>
# Search plugin center by name/description keywords
```

Center index default: `https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml`

You can override via:
```bash
lyenv config set plugins.registry_url <URL> --type=string
```

#### 3.4 Plugin Install / Add / Update / Info / List / Remove

```bash
lyenv plugin add <PATH> [--name=<INSTALL_NAME>]
# Install local directory plugin under custom install name

lyenv plugin install <NAME|PATH> [--name=<INSTALL_NAME>] [--repo=<org/repo>] [--ref=<branch|tag|commit|version>] [--source=<url>] [--proxy=<url>]
# Install from local path, remote repo, source archive or center name
# - NAME only: resolve from center; prefer archive+sha256 if present, else monorepo subpath

lyenv plugin update <INSTALL_NAME> [--repo=<org/repo>] [--ref=<branch|tag|commit|version>] [--source=<url>] [--proxy=<url>]
# Update installed plugin (git/center source)

lyenv plugin info <INSTALL_NAME|LOGICAL_NAME>
# Show manifest details, resolved directory and shims

lyenv plugin list [--json]
# List installed plugins (JSON for machine-readable)

lyenv plugin remove <INSTALL_NAME> [--force]
# Uninstall plugin and remove related shims
# If shell still resolves shim name after removal, run: hash -r
```

**Notes**:

- Shims bind to the install name (physical directory under plugins/).
- Shims prefer env var `LYENV_BIN` path; fallback to lyenv in PATH.
- Windows shims `.cmd/.ps1` also supported (generation carried but tested here on Linux).

#### 3.5 Run (Single/Multi-step, shell/stdio, Timeout/Policy)

```bash
lyenv run <PLUGIN> <COMMAND> [--merge=override|append|keep] [--timeout=<sec>] [--fail-fast|--keep-going] [-- ...args]
# Execute plugin command

# Examples:
lyenv run testtools run --merge=override --keep-going
lyenv run testtools slow --timeout=5 --fail-fast
```

- **shell**: Runs `bash -c "<program + args>"`. Captures stdout/stderr into JSON Lines logs.
- **stdio**: Sends a JSON request to stdin; expects JSON response with:
  - `status` (e.g., ok),
  - `logs` (array of strings echoed to console),
  - `artifacts` (array of paths),
  - `mutations`:
    - `global` (merged into lyenv.yaml),
    - `plugin` (merged into plugin-local config; original format preserved YAML/JSON by extension).

**Multi-step**: Compose multiple steps (shell/stdio mixed) with `continue_on_error`. Global `--keep-going` overrides per-step; `--fail-fast` stops on first error.

**Timeout**: Global deadline (sec). Uses `exec.CommandContext` so child processes are canceled when deadline is reached.

---

### 4. Manifests and Execution Model

#### 4.1 Manifest Files (YAML or JSON)

Plugin manifests support both formats and the following fields (common subset):

- `name` (string, required)
- `version` (string, required)
- `expose` (array of shim names, required)
- `config.local_file` (optional path to plugin-local config; YAML or JSON by extension)
- `commands`: array of command specs:
  - `name` (string, required, unique)
  - `summary` (string)
  - Either:
    - **Single command**:
      - `executor` (shell or stdio)
      - `program` (string; command or plugin-relative path)
      - `args` (array of strings)
      - `workdir` (string, plugin-relative or absolute)
      - `env` (map of string environment variables)
      - `use_stdio` (bool; for stdio)
    - **Or multi-step**:
      - `steps`: array of sub-commands with same fields per step, plus `continue_on_error` (bool)
- `entry`: optional default stdio entry:
  - `type`: "stdio"
  - `path` (string)
  - `args` (array of strings)

#### 4.2 shell vs stdio

- **shell**: best for simple commands without structured return. Logs are captured automatically.
- **stdio**: best for structured exchange:
  - Request JSON includes `action`, `args`, `paths`, `system`, `config`, `merge_strategy`, `started_at`.
  - Response JSON can include `mutations` to be merged safely by core with specified strategy (override/append/keep).

#### 4.3 Permissions and Logs

**Install/update normalize permissions**:
- Directories: 0755,
- Regular files: 0644,
- Files with shebang (`#!/...`): 0755.

**Logs**:
- Per plugin command: `plugins/<INSTALL_NAME>/logs/YYYY-MM-DD/<COMMAND>-<TIMESTAMP>.log` (JSON Lines: info, stdout, stderr, etc.).
- Global dispatch log: `.lyenv/logs/dispatch.log`.

---

### 5. Plugin Center (Monorepo + Archive+SHA‑256)

#### 5.1 Monorepo Structure

A single repository hosts many plugins in `plugins/<NAME>/.` The center index (YAML/JSON) maps `<NAME>` to either:

- `repo`, `ref`, `subpath` (monorepo checkout),
- or `versions[<ver>].source` (ZIP/TGZ URL) + `versions[<ver>].sha256` for archive distribution.

Center index example (YAML):
```yaml
apiVersion: v1
updatedAt: 2026-01-01T12:00:00Z
plugins:
  tester:
    desc: "Mixed steps demo"
    repo: "systemnb/lyenv-plugin-center"
    subpath: "plugins/tester"
    ref: "main"
    shims: ["tctl"]
    versions:
      "0.1.0":
        source: "https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/artifacts/tester-0.1.0.zip"
        sha256: "<64 hex sha>"
        shims: ["tctl"]
```

#### 5.2 Installation Resolution

- **NAME only**: lyenv will resolve from center:
  - If `source+sha256` present → download, verify SHA‑256, extract, install.
  - Else → clone monorepo (shallow) and copy subpath.

#### 5.3 CI in Center Repo

Center repo workflow (PR-based) generates:
- `artifacts/<NAME>-<VERSION>.zip` with all plugin files,
- `index.yaml` with source and sha256 entries,
- creates a PR; upon merging, source raw URLs are valid.

---

### 6. CI: E2E Testing (GitHub Actions)

We provide a workflow (`.github/workflows/e2e.yml`) that:

1. Checks out and builds lyenv,
2. Sets up Python (PyYAML) for YAML parsing,
3. Adds `dist/` to PATH,
4. Runs `scripts/full_e2e_test.sh`:
   - Environment create/init/activate,
   - Center sync/search,
   - Install (center name),
   - Run (shell+stdio, multi-step),
   - Verify logs/mutations,
   - Timeout/fail-fast,
   - Uninstall and shim removal,
   - Local plugin add/run/remove,
   - Archive+SHA‑256 validation (if center provides),
   - Config dump/load (JSON),
   - Dispatch log inspection,
5. Uploads logs as artifacts.

---

### 7. Troubleshooting

- **Shim still present after removal**: flush shell cache `hash -r`; check `type -a <shim>` / `which -a <shim>` for other instances in PATH.
- **`fork/exec ... no such file or directory` for stdio script**:
  - Ensure the script has executable bit (`chmod +x`) and LF line endings,
  - Proper shebang (`#!/usr/bin/env python3`),
  - `python3` in PATH (consider `manifest.env.PATH`).
- **Timeouts**: `context deadline exceeded` indicates global deadline; increase `--timeout` or reduce step durations.
- **Center index missing archive entries**: run center CI to generate `artifacts/*.zip` and `index.yaml` with `source+sha256`, then merge PR.

---

### 8. Contributing

1. Add plugins to center repo under `plugins/<NAME>/` with `manifest.yaml|yml|json` and required files.
2. Center CI will generate artifacts and index via PR.
3. For local development:
   ```bash
   lyenv plugin add ./plugins/<NAME> --name=<INSTALL_NAME>
   lyenv run <INSTALL_NAME> <COMMAND>
   ```

---

### 9. License

This project is licensed under the terms in `LICENSE`.

---

