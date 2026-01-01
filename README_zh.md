## 中文版本

### 1. 项目是什么（意义与目标）

**lyenv** 是一个基于目录结构的轻量环境管理工具，主要目标：

- 在单一目录下创建和激活隔离工作区（`bin/`、`plugins/`、`workspace/`、`.lyenv/`）。
- 以插件（Plugins）形式运行任何语言的程序：
  - **shell 执行器**：执行命令行，自动捕获日志。
  - **stdio 执行器**：通过标准输入/输出交换 JSON，实现结构化结果与安全的配置变更（`mutations`）。
- 将插件返回的结构化变更安全地合并到：
  - 全局配置 `lyenv.yaml`，
  - 插件私有配置 `plugins/<INSTALL_NAME>/config.yaml`（或 `.json`）。
- 记录每次运行的 **JSON Lines 日志** 与全局 **dispatch 日志**，便于审计与回放。
- 支持通过插件中心（Monorepo 子目录 + 归档分发）按名称安装插件，归档分发支持 **SHA‑256 校验**确保完整性。

**意义**：

- **与语言无关**：只要能在 shell 中运行或通过 stdio 读写 JSON，即可接入。
- **可复用、可追踪**：目录化 + JSON 日志，简化调试与审计。
- **结构化编排**：stdio 提供结构化结果与安全变更，避免脆弱的纯文本解析。
- **可规模分发**：插件中心支持大量插件在同一仓库管理，或以归档方式高效分发。

---

### 2. 快速开始

#### 2.1 构建

```bash
make build
# 或
go build -o ./dist/lyenv ./cmd/lyenv
```

#### 2.2 创建、初始化、激活

```bash
./dist/lyenv create ./my-env
./dist/lyenv init ./my-env
cd ./my-env
eval "$("./../dist/lyenv" activate)"
```

默认写入：

- 目录结构：`.lyenv/`、`.lyenv/logs`、`.lyenv/registry`、`bin/`、`cache/`、`plugins/`、`workspace/`。
- `lyenv.yaml` 包含插件中心索引默认值（见英文部分示例）。
- `.lyenv/registry/installed.yaml` 初始化为 `plugins: []`。

---

### 3. 命令教程与使用

输出均为英文。以下为主要命令：

#### 3.1 环境管理

```bash
lyenv create <DIR>
lyenv init <DIR>
lyenv activate  # eval "$(lyenv activate)"
```

#### 3.2 配置管理

```bash
lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]
lyenv config get <KEY>
lyenv config dump [<KEY>] <FILE>
lyenv config load <FILE> [--merge=override|append|keep]
lyenv config importjson <FILE> <JSON_KEY> [...]
lyenv config importyaml <FILE> <YAML_KEY> [...]
```

#### 3.3 插件中心与搜索

```bash
lyenv plugin center sync
lyenv plugin search <KEYWORDS...>
```

默认中心索引：`https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml`

可通过 `lyenv config set plugins.registry_url <URL> --type=string` 修改。

#### 3.4 安装 / 本地添加 / 更新 / 信息 / 列表 / 移除

```bash
lyenv plugin add <PATH> [--name=<INSTALL_NAME>]
lyenv plugin install <NAME|PATH> [--name=...] [--repo=...] [--ref=...] [--source=...] [--proxy=...]
lyenv plugin update <INSTALL_NAME> [--repo=...] [--ref=...] [--source=...] [--proxy=...]
lyenv plugin info <INSTALL_NAME|LOGICAL_NAME>
lyenv plugin list [--json]
lyenv plugin remove <INSTALL_NAME> [--force]
```

**说明**：

- Shim 用安装名绑定，优先使用 `LYENV_BIN` 指定的 lyenv 路径。
- 移除后若 shell 仍解析到旧的 shim，请运行 `hash -r` 刷新缓存。

#### 3.5 运行（单条 / 多步骤，shell / stdio，超时与策略）

```bash
lyenv run <PLUGIN> <COMMAND> [--merge=...] [--timeout=<sec>] [--fail-fast|--keep-going] [-- ...args]
```

- **shell**：适合无结构化返回的简单命令。
- **stdio**：核心向 stdin 写请求 JSON；插件从 stdout 返回 JSON（含 `mutations`），由核心安全合并。
- **多步骤**：`steps` 支持 shell 与 stdio 混用；`continue_on_error` 控制容错；全局 `--keep-going` / `--fail-fast`。
- **超时**：全局超时（秒），超时会取消子进程。

---

### 4. 清单与执行模型

#### 4.1 清单（YAML/JSON）

支持字段见英文版说明；`commands` 支持单条或多步骤；`entry` 可作为默认 stdio。

#### 4.2 shell 与 stdio

- **shell**：执行命令，自动捕获日志。
- **stdio**：结构化交互，可返还 `logs`、`artifacts`、`mutations` 由核心安全合并。

#### 4.3 权限与日志

**权限归一化**：目录 0755、普通文件 0644、带 shebang 的文件 0755。

**日志**：
- 插件命令：`plugins/<INSTALL_NAME>/logs/YYYY-MM-DD/<COMMAND>-<TIMESTAMP>.log`（JSON Lines）。
- 全局：`.lyenv/logs/dispatch.log`。

---

### 5. 插件中心（Monorepo + 归档 + SHA‑256）

- **Monorepo 子目录**：`plugins/<NAME>/`。
- **归档分发**：`versions[<ver>].source`（ZIP/TGZ URL）+ `versions[<ver>].sha256` 校验。

**安装解析顺序**：
1. 若存在 `source+sha256` → 下载校验 → 解压安装。
2. 否则 → 克隆仓库并复制 subpath。

中心仓库 CI（PR 流程）会生成 `artifacts/*.zip` 与 `index.yaml`。

---

### 6. 持续集成（GitHub Actions）

提供 `.github/workflows/e2e.yml` ，每次 push/PR/手动触发运行 **完整 E2E 脚本**，并上传日志构件。详见英文版说明。

---

### 7. 常见问题

- **移除后 shim 仍存在**：`hash -r` 刷新 shell 缓存；`type -a` / `which -a` 检查 PATH。
- **stdio 报 `no such file or directory`**：检查可执行位、LF 行尾、shebang、`python3` 是否在 PATH。
- **超时**：提示 `context deadline exceeded`，调整 `--timeout` 或缩短步骤。
- **中心索引缺少归档信息**：运行中心仓库 CI，生成并合并 PR。

---

### 8. 贡献

1. 在中心仓库新增 `plugins/<NAME>/`，包含清单与文件。
2. CI 生成归档与索引，建立 PR；合并后可供安装。
3. 本地开发可用：
   ```bash
   lyenv plugin add ./plugins/<NAME> --name=<INSTALL_NAME>
   lyenv run <INSTALL_NAME> <COMMAND>
   ```

---

### 9. 许可证

请参见仓库根目录的 `LICENSE` 文件。
