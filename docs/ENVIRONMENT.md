# Phase 0 环境勘察

记录于 2026-11-21（Asia/Shanghai）。

| 检查项 | 结果 |
|---|---|
| 项目根 | `/Volumes/1TB/Downloads/git/PdmView/PdmView_Codex_Task_Pack`。外层只有解压目录和 Finder 元数据。 |
| 初始文件 | 启动提示、产品与架构规范、全部任务卡、状态表和模板。未发现应用代码、Go 模块或已有仓库。 |
| Git | Apple Git 2.50.1。已在任务包目录初始化 `main`，无配置远端。 |
| 首发平台 | Apple M4，16 GiB，macOS 15.8.1（24H32），`darwin/arm64`。 |
| 显示器 | L272U，5120×2880，等效 2560×1440，60 Hz，系统报告 Metal 3。 |
| 默认 Go | Go 1.25.0，`GOTOOLCHAIN=auto`，默认 `CGO_ENABLED=1`，`CC=clang`。 |
| 验证用 Go | `GOTOOLCHAIN=go1.27.4 go version` 返回 `go version go1.27.4 darwin/arm64`。由 Go 工具链缓存提供，系统默认 Go 仍是 1.25.0。 |
| 平台 SDK | Xcode 26.3（17C529），`xcrun` 返回 macOS SDK 26.2，Apple clang 17.0.0。 |
| MySQL 与 Docker | 本机有 MySQL 客户端 26.10.0，未发现本机 Docker 或 Podman CLI。本机 3306 监听者是 P2P 转发程序。经现有 `ubuntu-local` SSH 入口只读查询，Docker Server 28.1.1，正在运行的 `wordpress_mysql8` 使用 `mysql:8.4.11` 且属于 WordPress。当前没有已确认可用于 PdmView 的隔离 MySQL 8.0 实例。 |
| 真实 PDM | 在工作区递归按扩展名盘点，结果为 0 份。 |

## P0-01 验收

- `sh -n scripts/check-environment.sh` 通过。`sh scripts/check-environment.sh` 在本机退出码 0，报告工具链和样本盘点结果。
- 项目根的 Git 识别已核实。任务包原文件保留，`tasks/TASK_STATUS.md` 记录本次验收。
- 样本盘点仅使用文件名和字节数。`.local/fixtures/` 和 `.pdm` 文件已加入忽略规则。P3 需提供 2～3 份真实脱敏样本，覆盖多 Diagram、Package、主外键和索引，另附 PowerDesigner 对照信息。
- Docker 盘点仅查询版本与容器镜像。Phase 9 的 MySQL 8.0 集成测试待届时准备隔离实例。

## 复查命令

```sh
sh scripts/check-environment.sh
GOTOOLCHAIN=go1.27.4 go version
system_profiler SPDisplaysDataType -detailLevel mini
```
