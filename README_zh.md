# lyenv —— 基于目录的隔离环境管理器  
**支持可视化工作流 GUI**

> This README is provided in [English](README.md) and [中文](README_zh.md) (Chinese).  
> 语言切换：下方包含 [English](README.md) 和 [中文](README_zh.md)，内容一致。  
> License: See `LICENSE` at the repository root.

---

## 1. lyenv 是什么？

**lyenv** 是一个以“目录”为核心的环境管理与自动化系统，强调：

- **可复现执行**
- **跨语言插件**
- **结构化结果与日志**
- **使用 GUI 编写、CLI 执行的工作流模型**

> **CLI 是运行时  
> GUI 是工作流编译器**

---

## 2. lyenv 解决了什么问题？

- Shell 脚本难维护
- GUI 工具无法自动化
- 插件过重（容器）或过轻（脚本）
- 输出只是文本，没有结构

**lyenv 的核心答案是：**

✅ 环境就是目录  
✅ 工作流就是插件  
✅ 结果通过 stdio JSON 返回  
✅ GUI 生成真实可执行插件

---

## 3. 核心概念

### 3.1 环境 = 一个目录

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

环境可复制、可检查、可版本化。

### 3.2 插件 = 可执行工作流

插件由以下组成（语言不限）：

- `manifest.yaml` / `manifest.json`
- shell 或 stdio 执行器
- 多步骤（multi‑step）
- 配置与日志

### 3.3 stdio 模式（关键能力）

通过 stdin/stdout 交换 JSON：

- **输入**：结构化请求  
- **输出**：结构化结果  
- 支持配置变更、最终输出、日志

---

## 4. GUI：推荐的工作流编写方式

✅ **强烈推荐使用 GUI**

GUI **不是**独立运行时，而是：

- 把可视化流程**编译**成 lyenv 插件

### GUI 运行流程

1. 画布中编写流程  
2. Group 表示一个命令  
3. 点击 **Run**  
4. 自动导出插件  
5. 安装到环境  
6. 执行并实时显示日志  
7. 自动清理插件  
8. 日志永久保存

---

## 5. 快速开始

### 5.1 构建

```bash
make build
make build-gui
```

### 5.2 创建环境

```bash
lyenv create ./demo
lyenv init ./demo
cd demo
eval "$(lyenv activate)"
```

---

## 6. 启动 GUI

```bash
lyenv gui start --open
```

注册环境供 GUI 使用：

```bash
lyenv gui add ./demo --name=demo
```

---

## 7. GUI 中运行流程

1. 选择环境  
2. 绘制流程  
3. 按 Group 运行  
4. 输入参数  
5. 查看实时日志

---

## 8. 常用命令

```bash
lyenv gui add <DIR>
lyenv gui list
lyenv gui prune
lyenv run <PLUGIN> <COMMAND>
```

---

## 9. 日志与结果

- 单次运行日志：`.lyenv/logs/dispatch/<ID>.log`
- 全局调度日志：`.lyenv/logs/dispatch.log`

格式为 **JSON Lines**，适合机器处理。

---

## 10. 适用人群

- 自动化开发者
- 本地工作流管理
- GUI + CLI 一致性需求
- 厌倦脚本地狱的人

---

## 11. 设计哲学

- **目录即环境**
- **JSON 即接口**
- **GUI 是编译器**
- **CLI 是运行时**

---

## 12. 许可证

见 `LICENSE`。

---