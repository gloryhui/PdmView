# 任务状态

状态说明：`TODO` / `IN_PROGRESS` / `BLOCKED` / `REVIEW` / `DONE`

Phase 0 与 Phase 1 已于 2026-11-21 验收完成。Phase 2 按 P2-01 → P2-02 → P2-03 → P2-04 → P2-05 推进。

| ID | 任务 | 状态 | 负责人 | 提交/证据 | 遗留问题 |
|---|---|---|---|---|---|
| P0-01 | 仓库与环境勘察 | DONE | GPT | `23ea2fd`，`sh -n`、脚本实跑、忽略规则验证通过，见 `docs/ENVIRONMENT.md` | P3 待真实脱敏 PDM，P9 待隔离 MySQL 8.0 |
| P0-02 | Go/MyGo 版本与 API 调研 | DONE | GPT / DeepSeek | `0bccd7e`，`cmd/mygo-probe`，macOS Metal 实跑、2× HiDPI、画布输入、系统剪贴板、Gate 0 三档性能和 Windows x64 构建，见 `docs/MYGO_API_VERIFICATION.md` | 文件面板已验证显示和取消，成功选中文件待复核。Windows 桌面待实跑。 |
| P0-03 | 项目骨架与规范文件 | DONE | GPT | `go.mod`、`go.sum`、`Makefile`、README 和架构文档，`make fmt`、`make check`、`go mod verify` 通过 | 正式应用与领域包按路线图落地 |
| P1-01 | Native UI 最小窗口 | DONE | DeepSeek / GPT | `519cd98`，macOS 原生启动、尺寸限制、关闭会话与 Go 检查通过，见 `docs/PHASE_1_ACCEPTANCE.md` | |
| P1-02 | Painter/Canvas API 验证 | DONE | DeepSeek / GPT | `48b147f`，曲线、2× 原生 GPU 截图、缩放与裁剪、Canvas 复用断言通过，见 `docs/PHASE_1_ACCEPTANCE.md` | |
| P1-03 | 三栏布局原型 | DONE | DeepSeek / GPT | 双语三栏、分隔线拖动、最小窗口截图、切换保留栏宽，通过 `make check`，见 `docs/PHASE_1_ACCEPTANCE.md` | |
| P1-04 | 界面国际化基础与语言偏好 | DONE | DeepSeek / GPT | `6f01d72`，双语资源、偏好单测、macOS 菜单双向切换、重启恢复、保存失败实测通过，见 `docs/PHASE_1_ACCEPTANCE.md` | Windows 实机列入 P10-02 |
| P2-01 | 模拟图数据生成器 | DONE | DeepSeek / GPT | `go test -count=1 ./internal/graph`、`go vet ./internal/graph`、格式检查通过。固定 seed `20261121`，覆盖 50/150/300/500 节点、双密度、双语长名、交叉关系和端口 | |
| P2-02 | 坐标变换与缩放平移 | IN_PROGRESS | DeepSeek / GPT | 世界坐标、视口局部 DIP 与窗口 DIP 的双向换算 | |
| P2-03 | HitTest 与空间索引 | TODO | DeepSeek | | |
| P2-04 | Viewport Culling 与 LOD | TODO | DeepSeek | | |
| P2-05 | 性能基准与报告 | TODO | GPT | | |
| P3-01 | 真实 PDM 样本盘点 | TODO | GPT | | |
| P3-02 | XML 结构与版本研究 | TODO | GPT | | |
| P3-03 | 原始 XML 表示 | TODO | DeepSeek | | |
| P3-04 | Model/Package/Table 解析 | TODO | DeepSeek | | |
| P3-05 | Column/Key/Index/Reference 解析 | TODO | DeepSeek | | |
| P3-06 | Diagram/Symbol/路径解析 | TODO | DeepSeek | | |
| P3-07 | 诊断与解析测试 | TODO | DeepSeek | | |
| P4-01 | Domain Model 冻结 | TODO | GPT | | |
| P4-02 | Diagram 到 Graph 映射 | TODO | DeepSeek | | |
| P4-03 | 原始布局核对 | TODO | GPT | | |
| P5-01 | Graph Node/Edge | TODO | DeepSeek | | |
| P5-02 | World/View/Screen 变换 | TODO | DeepSeek | | |
| P5-03 | R-Tree/空间查询 | TODO | DeepSeek | | |
| P5-04 | 命中测试 | TODO | DeepSeek | | |
| P5-05 | 关系线几何与 fallback | TODO | DeepSeek | | |
| P6-01 | 表节点绘制 | TODO | DeepSeek | | |
| P6-02 | 关系线绘制 | TODO | DeepSeek | | |
| P6-03 | LOD/缓存/裁剪 | TODO | DeepSeek | | |
| P6-04 | 多画板与视口工具 | TODO | GPT | | |
| P6-05 | 画布性能验收 | TODO | GPT | | |
| P7-01 | 主窗口三栏布局 | TODO | GPT | | |
| P7-02 | 模型树与 Package | TODO | DeepSeek | | |
| P7-03 | 表详情与字段列表 | TODO | DeepSeek | | |
| P7-04 | 搜索与定位 | TODO | DeepSeek | | |
| P7-05 | 完整信息复制 | TODO | DeepSeek | | |
| P8-01 | MySQL 类型映射 | TODO | GPT | | |
| P8-02 | 单表 DDL | TODO | DeepSeek | | |
| P8-03 | 索引/主键/约束 | TODO | DeepSeek | | |
| P8-04 | 全库依赖与外键策略 | TODO | GPT | | |
| P8-05 | 全库初始化 SQL | TODO | DeepSeek | | |
| P9-01 | SQL 预览与复制 | TODO | DeepSeek | | |
| P9-02 | SQL 文件导出 | TODO | DeepSeek | | |
| P9-03 | Golden Tests | TODO | DeepSeek | | |
| P9-04 | MySQL 8.0 集成测试 | TODO | GPT | | |
| P10-01 | macOS 验收与打包 | TODO | GPT | | |
| P10-02 | Windows x64 验收 | TODO | GPT | | |
| P10-03 | 文档与发布检查 | TODO | GPT | | |
