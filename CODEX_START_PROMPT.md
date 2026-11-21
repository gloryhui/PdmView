你现在接手一个新项目：PdmView。请先完整阅读仓库根目录中的 `README.md`、`PROJECT_BRIEF.md`、`ARCHITECTURE.md`、`AGENT_WORKFLOW.md`、`QUALITY_GATES.md`，以及 `tasks/00_ROADMAP.md` 和各 Phase 任务文件。不要跳过文档，也不要先写一堆代码。

# 你的角色与模型分工

当前协作模型：
- 主 Agent：GPT-6.2-Sol-High。你负责技术负责人、架构、接口决策、代码审查、集成和最终验收。
- 执行 Agent：DeepSeek-V4.1-Flash-1017。由你派发边界明确的编码任务，负责主要模块实现和测试。

必须遵守 `AGENT_WORKFLOW.md`：
- 先接口、后实现。
- 同一文件同一时间只能由一个 Agent 修改。
- 执行 Agent 不得擅自更改公共接口。
- 任务完成必须提交修改文件、测试命令、测试结果、风险和遗留问题。
- 不允许未经审查就合并。
- 如果 Codex 支持隔离 worktree，优先使用；否则串行交接。
- 不要无限拆分子 Agent，最多一个执行 Agent。

# 项目目标

开发 Go + MyGo Native UI 的 PowerDesigner `.pdm` 查看器，支持保存表和字段的基本文字信息。

第一版必须实现：
1. 打开 PDM。
2. 解析模型、Package、表、字段、主键、外键、索引。
3. 解析多个 Diagram，尽可能还原 PowerDesigner 原始画板布局、表节点位置尺寸、关系路径。
4. 大型 ER 画布：平移、缩放、适应窗口、显示全部、选中表。
5. 点击表查看详细字段、索引、主键和外键信息。
6. 一键复制完整表信息。
7. 一键复制单表 MySQL CREATE TABLE SQL。
8. 生成全库初始化 SQL：CREATE DATABASE、USE、所有表、字段注释、表注释、主键、索引、外键。
9. SQL 预览、复制和导出 `.sql`。
10. 简体中文与英文界面切换，从 Phase 1 建立文案资源、原生设置入口和本地语言偏好。默认语言和覆盖范围以 `PROJECT_BRIEF.md` 为准。
11. 在原生表属性窗口将现有表和字段的名称、编码与注释直接保存到打开的 PDM。

明确不做：表结构编辑、数据库连接、执行 SQL、数据库同步、AI 助手、WebView、Electron、Tauri、网页前端。

# 技术路线

- Go + MyGo Native UI。
- 应用界面完全使用原生 UI，不创建 WebView。macOS 系统框架加载按 [架构规范](ARCHITECTURE.md#原生-ui-与平台运行时边界) 记录。
- Lobe MyGo 仅在确认与当前 MyGo Native UI 兼容后复用，不得引入 Web 前端运行时。
- PDM Parser、Domain Model、Graph Engine、Renderer、SQL Generator、UI 必须分层。
- 真实 PDM 原始 Diagram 布局优先；缺失时才 fallback 自动布局。
- 视口裁剪、LOD、空间索引、缓存必须进入正式画布设计。
- 单表 SQL 和全库 SQL 必须共用 Generator。
- 未知 PDM 结构、未知 MySQL 类型不得静默丢弃或擅自降级。

# 重要：Go/MyGo 版本不能猜

先检查当前环境和 MyGo 官方文档/源码，确认 Go 版本要求、MyGo 当前 API、Lobe MyGo 兼容性。不要根据旧对话或旧文档盲目锁定版本。所有 Painter、Canvas、Transform、Clip、事件、剪贴板、文件选择器 API 必须实际验证并编译运行，不能凭空猜 API。

# 当前执行规则

你现在只开始 Phase 0，不要跳到正式 UI 或完整业务实现。

## Phase 0 任务

1. 检查仓库目录、Git 状态、已有代码，禁止覆盖已有用户文件。
2. 检查 Go 环境、平台 SDK、CGO、编译工具。
3. 检查是否存在真实 PDM 样本，只记录本地路径和元信息，不输出敏感内容。
4. 调研 MyGo Native UI 当前官方文档、仓库和示例。
5. 确认原生窗口、Painter、Canvas、Transform、Clip、Path、Text、鼠标事件、DPI、剪贴板和文件选择器能力。
6. 确认 macOS Metal、Windows D3D11 的构建要求。
7. 确认 Lobe MyGo 的组件是否属于原生 UI，是否兼容当前版本；如果不适合，明确说明并使用 MyGo 原生控件，不要硬接。
8. 创建项目骨架和规范文档。
9. 建立 `docs/MYGO_API_VERIFICATION.md`、`docs/ENVIRONMENT.md`、`docs/DECISIONS.md`。
10. 建立并维护 `tasks/TASK_STATUS.md`。

## Phase 0 完成后

先汇报：
- 环境检查结果。
- Go/MyGo/Lobe MyGo 的实际版本。
- 官方 API 证据和最小程序验证结果。
- 是否实测窗口使用原生 UI 且未创建 WebView。
- 真实 PDM 样本是否可用。
- 项目骨架文件清单。
- 测试结果。
- 风险与待用户提供材料。

只有 Phase 0 通过后，才进入 Phase 1：Native UI 与 Canvas 原型。

# 施工质量要求

- 每个任务必须按任务卡执行并更新状态。
- 每次修改后运行相关测试。
- 关键阶段运行 `gofmt`、`go test ./...`、`go vet ./...`。
- 不允许只创建空文件或 TODO 占位。
- 不允许 Mock 结果冒充真实 PDM 解析。
- 不允许声称性能达标而没有基准数据。
- 不允许声称 SQL 可执行而没有 MySQL 8.0 集成测试。
- 不允许为通过测试删除断言。
- 不允许把真实业务 PDM 上传到公共服务或公开仓库。
- 发现需求歧义时先记录具体问题和建议；能安全继续的工作不要停摆。
- 不要每完成一个小任务就询问用户；按路线图持续推进。
- 但在缺少真实 PDM 样本导致无法验证解析结构时，必须明确列出所需样本，不得虚构结论。

现在开始 Phase 0。先勘察和验证，再编码。
