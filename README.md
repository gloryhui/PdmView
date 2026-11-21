# PdmView

**项目目标：** Go + MyGo Native UI 的 PowerDesigner `.pdm` 只读查看器。  
**首版功能：** 读取 PDM、还原原始画板布局、查看表及字段、复制完整表信息、复制单表 MySQL SQL、生成全库初始化 SQL（含表注释、字段注释、索引、主外键）。

首版还包含简体中文与英文界面切换。默认按系统语言选择，可从原生菜单切换并在重启后恢复选择。详见 [产品范围](PROJECT_BRIEF.md#首版界面语言)。

## 开发与验证

在本目录运行命令。项目要求 Go 1.27.3，已用 Go 1.27.4 和 MyGo `v0.10.0` 验证。默认准备 macOS Apple Silicon 开发环境。

```sh
make env    # 复查机器、构建工具和本地样本元信息
make fmt    # 格式化 Go 源码
make check  # 验证格式，运行 go test ./... 和 go vet ./...
make probe  # 启动 Phase 0 独立原生探针
```

`cmd/mygo-probe/` 是技术验证入口，显示两张模拟表。正式应用的目录边界见 [ARCHITECTURE.md](ARCHITECTURE.md)，验证步骤见 [MyGo API 验证](docs/MYGO_API_VERIFICATION.md)，任务进展见 [任务状态](tasks/TASK_STATUS.md)。

真实样本放在 `.local/fixtures/`。本地日志、运行截图和二进制放在 `.out/`。

本机 Phase 0 验收使用 macOS arm64 和 `CGO_ENABLED=0`。`make fmt`、`make check` 和 `go mod verify` 均通过。原生探针可以运行，Windows x64 已交叉构建。

## 使用方法

1. 将本压缩包解压到项目仓库根目录，或将其中内容复制到现有仓库。
2. 准备 2～3 份真实 PDM 样本，放入本地测试目录。**不要将真实业务 PDM 提交到公开仓库。**
3. 在 Codex 中选择项目目录，使用根目录的 `CODEX_START_PROMPT.md` 作为启动 Prompt。
4. 主 Agent 使用 GPT-6.2-Sol-High；执行 Agent 使用 DeepSeek-V4.1-Flash-1017。
5. 按 `tasks/00_ROADMAP.md` 顺序推进，不允许跳过 Phase 0 技术验证。
6. 每个任务完成后更新 `tasks/TASK_STATUS.md`，记录提交、测试结果、风险和遗留问题。

## 文档索引

- `CODEX_START_PROMPT.md`：直接粘贴给 Codex 的启动指令。
- `PROJECT_BRIEF.md`：产品范围、技术边界和验收口径。
- `ARCHITECTURE.md`：模块边界、数据流、关键设计约束。
- `AGENT_WORKFLOW.md`：双模型分工、交接和冲突处理。
- `QUALITY_GATES.md`：阶段门禁、测试及性能验收。
- `tasks/00_ROADMAP.md`：阶段顺序与依赖。
- `tasks/TASK_STATUS.md`：任务状态登记表。
- `tasks/PHASE_0...PHASE_6.md`：逐项施工任务及验收标准。
- `templates/AGENT_HANDOFF.md`：执行 Agent 交接模板。
- `templates/BUG_REPORT.md`：问题报告模板。
- `templates/DECISION_RECORD.md`：技术决策记录模板。

## 重要约束

- 第一版只读，不编辑、不保存回 PDM。
- 先验证 MyGo Native UI 和绘制 API，再开发正式界面。
- 原始 Diagram 布局优先；只有缺少有效布局时才使用自动布局。
- 未知 XML 节点、属性和对象引用不得静默忽略，必须产生诊断。
- SQL 生成必须有明确的类型映射规则；未知类型不得擅自降级成 VARCHAR。
- 单表 SQL 与全库 SQL 必须共用同一套 Generator。
- 不把 Draw 回调当作业务计算入口；布局、解析、文本测量和空间索引更新必须在绘制前完成。
- 不允许用模拟 PDM 解析结果冒充真实 PDM 兼容性。
