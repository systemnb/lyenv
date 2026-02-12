# lyenv — Directory‑based Isolated Environment Manager  
**with Visual Workflow GUI**

> This README is provided in **English** and **中文**.  
> License: see `LICENSE`.

---

## 1. What is lyenv?

**lyenv** is a **directory‑based isolated environment manager** with a strong focus on:

- **Reproducible execution**
- **Language‑agnostic plugins**
- **Structured, traceable automation**
- **Visual workflow authoring via GUI**

At its core, lyenv treats **a directory as an environment**, and **a workflow as a plugin**.

> **CLI is the runtime.  
> GUI is the workflow compiler.**

---

## 2. Why lyenv?

Traditional tools often fall into one of these traps:

- Scripts are easy to write but hard to maintain
- GUI tools are easy to click but hard to automate
- Plugins are either too heavyweight (containers) or too weak (shell snippets)
- Outputs are text logs instead of structured results

**lyenv solves this by combining:**

✅ Directory‑based environments  
✅ Structured plugin execution (stdio JSON)  
✅ Deterministic logs and dispatch records  
✅ A GUI that *generates real CLI plugins*, not a toy runtime

---

## 3. Core Concepts

### 3.1 Environment = Directory

An environment is simply a directory created by lyenv:

```text
my-env/
├─ bin/
├─ plugins/
├─ workspace/
├─ cache/
├─ .lyenv/
│  ├─ logs/
│  ├─ registry/
│  └─ dispatch.log
└─ lyenv.yaml
```

This makes environments:

- Portable
- Inspectable
- Easy to back up or version‑control

### 3.2 Plugin = Executable Workflow

A plugin is a directory with a `manifest.yaml|json` describing:

- Commands
- Executors (shell or stdio)
- Multi‑step execution
- Exposed shims
- Config files

Plugins can be written in any language.

### 3.3 stdio Execution (Key Feature)

In stdio mode, plugins communicate with lyenv via JSON:

- **Input:** JSON via stdin  
- **Output:** JSON via stdout

This enables:

- Structured logs
- Returning final results
- Safe configuration mutations
- Deterministic behavior (no log scraping)

---

## 4. The GUI: Visual Workflow Compiler (Recommended)

✅ **Recommended way to build workflows**

The GUI is **not** a separate runtime.  
It is a visual compiler that turns workflows into **real lyenv plugins**.

### 4.1 What the GUI Does

In the GUI you can:

- Draw workflows using nodes and edges
- Group nodes into logical workflow groups
- Each group becomes one CLI command
- Export the workflow as a multi‑step stdio plugin

### 4.2 Run Flow (GUI → CLI → Logs)

When you click **Run** in the GUI:

1. Choose a workflow group  
2. GUI exports the workflow into a temporary plugin  
3. Plugin is installed into the selected lyenv environment  
4. You enter runtime arguments  
5. lyenv executes the plugin via stdio  
6. Logs stream in real‑time via WebSocket  
7. Plugin is automatically cleaned up  
8. Logs are persisted under `.lyenv/logs/dispatch/`

✅ The same plugin can be run later via CLI if exported explicitly.

---

## 5. Quick Start

### 5.1 Build

```bash
make build
make build-gui
```

### 5.2 Create an Environment

```bash
lyenv create ./demo
lyenv init ./demo
cd demo
eval "$(lyenv activate)"
```

---

## 6. Start the GUI

```bash
lyenv gui start --open
```

Register an environment for GUI usage:

```bash
lyenv gui add ./demo --name=demo
```

---

## 7. Running Workflows (GUI)

1. Open GUI in browser  
2. Select an environment  
3. Draw your workflow  
4. Group nodes into workflows  
5. Click **Run**  
6. Select workflow group  
7. Enter arguments  
8. Watch logs in the built‑in console  

Logs are saved even if the temporary plugin is auto‑cleaned.

---

## 8. CLI Usage (Core Commands)

### Environment

```bash
lyenv create <DIR>
lyenv init <DIR>
lyenv activate
```

### GUI Environment Registry

```bash
lyenv gui add <DIR> [--name=<NAME>]
lyenv gui list
lyenv gui remove <NAME|PATH>
lyenv gui prune
```

### Plugins

```bash
lyenv plugin add <PATH>
lyenv plugin install <NAME>
lyenv plugin list
lyenv plugin remove <INSTALL_NAME>
```

### Run

```bash
lyenv run <PLUGIN> <COMMAND> [--json] [--timeout=SEC]
```

---

## 9. Logs and Observability

- Per‑run logs: `.lyenv/logs/dispatch/<DISPATCH_ID>.log`
- Global dispatch log: `.lyenv/logs/dispatch.log`

Logs are **JSON Lines**, suitable for tooling and automation.

---

## 10. Who is lyenv for?

- Developers building repeatable automation
- Teams needing auditable local workflows
- Tool authors who want GUI + CLI consistency
- Anyone tired of fragile shell pipelines

---

## 11. Philosophy

- **Directories** over databases  
- **JSON** over text  
- **Workflows as code**  
- **GUI as compiler, CLI as runtime**

---

## 12. License

See `LICENSE`.

---
