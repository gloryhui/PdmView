# 任务状态

状态说明：`TODO` / `IN_PROGRESS` / `BLOCKED` / `REVIEW` / `DONE`

Phase 0、Phase 1 与 Phase 2 已于 2026-11-21 验收完成。Phase 3 的解析器、诊断和 XML 对照已完成。Gate 1 的 PowerDesigner 应用内对照待验收。

Phase 4 的领域模型冻结、映射及 XML 对照已完成。P4-03 的 PowerDesigner 应用内对照待本地 Windows 客户端验收，详见 [Phase 4 验收记录](../docs/PHASE_4_ACCEPTANCE.md)。

Phase 5 五项任务已于 2026-11-21 完成并通过 [图引擎验收](../docs/PHASE_5_ACCEPTANCE.md)。

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
| P2-02 | 坐标变换与缩放平移 | DONE | DeepSeek / GPT | `go test -count=1 ./internal/graph`、`go vet ./internal/graph`、格式检查通过。覆盖非零视口原点、反向变换、DIP 平移、缩放限位锚点、适应与 500 节点显示全部 | |
| P2-03 | HitTest 与空间索引 | DONE | DeepSeek / GPT | 单测、vet、格式检查通过。四种规模和每档 71 个查询区域与线性扫描一致。500 节点局部查询 7.90 → 1.47 µs，全图查询 5.68 → 0.60 µs | |
| P2-04 | Viewport Culling 与 LOD | DONE | DeepSeek / GPT | `make check`、e2e 标签测试通过。覆盖穿越关系、视口外跳过字段、0/2/5 行及双阈值回滞。macOS Metal 2× 原生窗口验证命中、拖动、滚轮锚点、适应选中和全图 | 长文字按卡片可用宽度预排版省略号 |
| P2-05 | 性能基准与报告 | DONE | DeepSeek / GPT | [验收记录](../docs/CANVAS_BENCHMARK.md)。macOS Metal 2×，密集与稀疏布局各四组 10 秒长测全部完成。150/300 节点均达 60 FPS 目标，500 节点密集/稀疏分别为 60.70/60.79 FPS。`make check` 及原生基准构建通过 | |
| P3-01 | 真实 PDM 样本盘点 | DONE | GPT | 本地只读盘点及 SHA-256 复核通过。132,997 字节、UTF-8、16.5.0.3982、1 画板，详见 `docs/PDM_FORMAT.md` | 仅一份真实样本 |
| P3-02 | XML 结构与版本研究 | DONE | GPT | Expat 盘点、ElementTree 结构核对及 `xmllint --nonet --noout` 通过。已对照 12.5、15.1、16.5 公开导出的 XML 结构，详见 `docs/PDM_FORMAT.md` | 画板单位及其他版本待实机对照 |
| P3-03 | 原始 XML 表示 | DONE | DeepSeek / GPT | `go test -count=1 ./internal/pdm`、`go vet ./internal/pdm` 通过。本地样本解码得到 190 个定义，原始层诊断 0 条 | 支持 UTF-8 XML |
| P3-04 | Model/Package/Table 解析 | DONE | DeepSeek / GPT | 包级测试、vet 通过。本地核对 1 模型、13 表、0 Package，DBMS、表顺序、Name/Code 和归属与 XML 一致。嵌套 Package 合成测试通过 | PowerDesigner 应用内对照待验证 |
| P3-05 | Column/Key/Index/Reference 解析 | DONE | DeepSeek / GPT | 包级测试、vet 通过。本地 149 字段及顺序、13 主键、13 非空标记、55 字段注释及 6 个 Precision 原值逐项对照 XML。合成测试覆盖复合索引和跨包外键 | 用户样本无索引、外键及自增 |
| P3-06 | Diagram/Symbol/路径解析 | DONE | DeepSeek / GPT | 包级测试、vet 通过。用户样本 1 画板、8 表图元、全部矩形及表引用对照一致。三份公开导出共 9 画板、37 表图元及 23 关系图元的引用和几何逐项核对 | 画板原始单位和应用内坐标对照待 P4-03 |
| P3-07 | 诊断与解析测试 | DONE | DeepSeek / GPT | `make check` 通过。`cmd/pdm-inspect` 本地样本 0 条诊断。31,431,470 字节合成文件实测 1,000 表、50,000 字段，0.30 秒、峰值 RSS 193.5 MiB。截断、未知属性、引用、重复 ID、Unicode、空值、多包多画板和输入上限均有验证 | |
| P4-01 | Domain Model 冻结 | DONE | GPT | `8f479ce`，ADR-004 按本地样本和公开导出冻结现有 `internal/pdm` 模型及映射契约。`go test -count=1 ./internal/pdm`、样本 SHA-256 及忽略规则复核通过 | |
| P4-02 | Diagram 到 Graph 映射 | DONE | DeepSeek / GPT | `c8dcfe1`，`MapDiagram` 保留图元身份、字段顺序、原始矩形和有序路径。`go test -count=1 ./internal/graph ./internal/pdm`、两包 `go vet` 和格式检查通过，覆盖重复表图元、跨包、端点解析和混合布局 | 缺失路径的自动路由留待 P5-05 |
| P4-03 | 原始布局核对 | BLOCKED | GPT | [验收记录](../docs/PHASE_4_ACCEPTANCE.md)。本地样本 8/8 图元的 X/Y、宽高和 90/90 字段顺序一致。三份公开 XML 的 23 条关系和 56 个路径点逐项一致。`make check`、实样本对照及 SHA-256 复核通过 | 缺可用的本地 PowerDesigner Windows 客户端，应用内对照与单位标定待验收。状态标志 `AutoLayout` 已交付，界面显示待接入正式画布 |
| P5-01 | Graph Node/Edge | DONE | DeepSeek / GPT | `bef71cf`，ADR-005。正式场景与合成数据分离，节点和边可按偏移追溯 Diagram、图元、模型对象和原始几何。Graph/PDM 定向测试及 vet 通过 | |
| P5-02 | World/View/Screen 变换 | DONE | DeepSeek / GPT | `46f42bb`，Graph 定向测试、vet 通过。在 1024 × 768 DIP 视口和 24 DIP 边距下，本地样本完整适应，8/8 图元中心往返通过，原型相机下限会截断全图 | PDM 物理单位标定随 P4-03 保留 |
| P5-03 | R-Tree/空间查询 | DONE | DeepSeek / GPT | `96f6415`，BVH 支持原子替换场景和点查询，定向测试、并发 race 验证及本地样本线性对照通过。500 节点点查询 298.1 → 33.56 ns，局部视口 8.034 → 1.650 µs，500 节点与 1,000 边索引重建 2.11 ms | 重建在交互查询路径外执行 |
| P5-04 | 命中测试 | DONE | DeepSeek / GPT | `4e6d7ca`，Graph 定向测试和 vet 通过，覆盖窗口偏移、节点优先及边的 DIP 容差。本地样本 8 图元在 0.5×、1×、2× 共 24/24 次命中，返回表和图元 ID | 调用方使用与画板同代的 BVH |
| P5-05 | 关系线几何与 fallback | DONE | DeepSeek / GPT | [图引擎验收](../docs/PHASE_5_ACCEPTANCE.md)。23 条源路径的 56 个点全部保留，内存副本清除路径后 23/23 条安全重建。四档密集图 2,000/2,000 条安全路由，500 节点/1,000 边中位数 106.29 ms。`make check`、图包 race 及样本校验通过 | 搜索超限或无安全路径时保留空显示几何 |
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
