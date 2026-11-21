# 技术决策

## ADR-001 原生桌面与工具链

- 日期：2026-11-21
- 状态：已采纳
- 项目模块使用 `pdmview`。项目尚无远端或可分配的发布模块路径。
- MyGo 锁定官方 tag `v0.10.0`（`ebde9399baf2734b3361856a53f4036859973d1d`）。`go.mod` 声明依赖要求的 Go 1.27.3，验证使用工具链 Go 1.27.4。桌面构建以 `CGO_ENABLED=0` 为基准。
- 所有窗口使用 `WindowOptions.Content` 和 `ui.View`。macOS arm64 作为实际验证平台，Windows x64 单独记录交叉构建与真实运行结果。
- 界面全部使用原生 UI，应用不创建 WebView。系统 WebKit 的启动加载按 [平台运行时边界](../ARCHITECTURE.md#原生-ui-与平台运行时边界) 记录。
- 选用 MyGo 原生控件开展后续工作。Lobe MyGo 与选定 MyGo 的兼容结果单独记录，Phase 0 只有探针所需的 MyGo 直接依赖。

依据：[Go 版本历史](https://go.dev/doc/devel/release#go1.27.0)、[MyGo v0.10.0 的 go.mod](https://github.com/egoist/mygo/blob/v0.10.0/go.mod)、[MyGo 原生窗口](https://github.com/egoist/mygo/blob/v0.10.0/docs/ui/README.md)。

## ADR-002 Phase 0 探针边界

- 日期：2026-11-21
- 状态：已采纳
- 隔离在 `cmd/mygo-probe`，以确定性模拟几何验证绘图、输入、系统服务和帧耗时。探针不承载 PDM 数据，也不定义后续业务公共接口。
- 以 DIP 作为窗口输入与绘制单位。手势在输入层转换为探针视口状态，绘制只使用已定义的几何和预先排版的文字。
- Gate 0 性能数据来自原生探针的简单合成场景。Phase 2 仍按自己的规模、数据集、采样方法与验收项验证交互和性能。
- 两节点保留 `StrokePath` 虚线曲线，用于验证 API。大规模合成基准使用不透明 `StrokePolyline`；首次实测中批量虚线路径耗尽遮罩图集并回退 CPU。正式关系线须保持原始路径语义，渲染策略留待 Phase 6 按真实数据重新验证。

## ADR-003 首版简体中文及英文界面

- 日期：2026-11-21
- 状态：设计已采纳，实施任务为 `P1-04`
- 首次启动根据系统首选界面语言选择简体中文或英文。已保存的显式语言选择优先，系统语言无法识别时使用英文。
- Phase 1 在最小原生窗口建立内置语言资源、语言状态、设置入口和本地偏好，验证即时切换和重启恢复。后续阶段通过同一文案入口交付各自界面。
- 语言选择作用于应用界面。PDM 原文、结构化复制文本及 SQL 保持稳定。产品规则见 [PROJECT_BRIEF.md](../PROJECT_BRIEF.md)，实现边界见 [ARCHITECTURE.md](../ARCHITECTURE.md)。

## ADR-004 冻结 PDM 领域模型和 Diagram 映射契约

- 日期：2026-11-21
- 状态：已采纳，P4-01
- 冻结 `internal/pdm` 中 `Parse`、`Project` 和 `Document` 的结构与语义。文档内 XML `Id` 是定义及引用的键，`ObjectID` 保留为独立元数据。`Model` 和嵌套 `Package` 各自持有 `Contents`，`DefaultDiagramID` 在所属 `Contents` 内解析。字段、键成员、索引成员、外键 Join、图元及几何点均保留输入顺序。
- `Name`、`Code` 各自保留。`OptionalText` 区分不存在和显式空文本，布尔指针保留未指定、明确为否和明确为是。类型、默认值和约束原始语义由消费者判断。缺失引用、未知结构及无效几何读取原有诊断，原始 XML 节点可追溯。外部 Shortcut 只保留引用，模型不补造表定义。
- `Diagram` 下的 `TableSymbol`、`ReferenceSymbol` 各自保留 XML 图元 ID、逻辑对象引用及归属。矩形、`ListOfPoints` 和显式 `ControlPoints` 使用原始有符号整数。同一表的不同图元独立存在。依据见 [PDM 格式研究](PDM_FORMAT.md)，包括本地 16.5 样本及公开 12.5、15.1、16.5 导出。
- 破坏性变更须先新增决策，记录样本依据、受影响消费者、迁移及测试，再修改契约。保持解析、映射和渲染的单向依赖。

### P4-02 接口

- `graph.MapDiagram(model *pdm.Model, diagram *pdm.Diagram) DiagramGraph` 生成一份画板结果。`DiagramGraph` 提供 `DiagramID string`、`Scene Scene`、`NodeSources []DiagramNodeSource`、`EdgeSources []DiagramEdgeSource`、`AutoLayout bool`。调用者负责选择 `Diagram`。`nil` 画板时使用模型及其嵌套 Package 中的表。空输入产生空图。
- `DiagramNodeSource` 包含 `SymbolID string`、`TableID string`、`Rect *pdm.Rect`；`DiagramEdgeSource` 包含 `SymbolID`、`ReferenceID`、`SourceSymbolID`、`DestinationSymbolID`（均为 `string`），以及 `Rect *pdm.Rect`、`Points` 和 `ControlPoints`（均为 `[]pdm.Point`）。来源数组与 `Scene` 节点、边按偏移一一对应。几何副本保留精确整数，互不共享可变数据。`Scene` 是浮点世界坐标，直接投影原始 X/Y，暂未标定与 DIP 的比例。`Scene.Bounds` 是节点及非空显示路径的并集。
- 显式画板按图元原序生成节点。节点和边的整数 `ID` 为各自画板内的切片偏移。标题和字段使用已解析的模型名称及字段原序（名称为空时取编码）；外部或未解析图元保留来源及位置，显示可用引用 ID。关系的 `From`、`To` 只从本画板的图元 ID 解析，未解析端点用 `-1`。保存有序原始路径和独立的控制点，不连缀两者；缺少路径时保持空 `Edge.Points`，自动路由留待 P5-05。
- 有表图元时只映射画板内的表图元。无表图元的显式画板回退至其归属 `Contents` 直接持有的表。缺失、反向或零尺寸矩形只对受影响的节点执行确定性网格布局。混合布局将自动放置的节点排在有效矩形联合范围右侧，使用同一坐标系，有效位置与尺寸原样保留。全回退采用独立的网格坐标。`AutoLayout` 在至少一个节点采用自动位置时为真，由后续正式画布呈现本地化的“自动布局”状态。

## ADR-005 正式图引擎与关系路由

- 日期：2026-11-21
- 状态：已采纳，Phase 5
- `Scene` 的交互索引是切片偏移。`DiagramGraph.NodeSource` 和 `EdgeSource` 用同一偏移读取原始对象和图元的来源视图，返回的矩形与点切片按只读数据使用。原始几何继续由 ADR-004 的来源数据保留。执行路径从 `BuildDiagramGraph` 开始，先调用 `MapDiagram`，再为已有关系但缺失路径的边计算显示几何。Phase 4 的 `MapDiagram` 保持纯映射。
- `Camera` 统一使用世界坐标、视口局部 DIP 和窗口 DIP。`ViewportBounds` 返回窗口内视口，`VisibleWorld` 返回其世界范围，`FitBounds` 适应世界边界。原 Phase 2 的 `NewCamera` 保留缩放约束，`NewDiagramCamera` 允许对跨度更大的 PDM 原始坐标缩小。比例仍为当前视图的 DIP/世界单位，物理单位标定等待 PowerDesigner 对照。
- 复用 Phase 2 的 BVH 精确查询。`SpatialIndex.ReplaceScene` 从同一份新场景批量重建边和节点快照，支持移动、增删或重新排序后的偏移映射。引擎按场景快照使用索引，更新、删除触发重建，指针移动只查询索引。`SpatialIndex.QueryPoint` 查询点覆盖的节点。`DiagramGraph.HitTest` 以窗口 DIP 和配套索引查询，复用节点优先及边距离规则，并返回原始表、关系及图元 ID；缺少索引时返回空命中。
- 已保存的关系路径优先，点序及控制点均保留。自动路由只使用已确认的关系和本画板端点；显式画板中的可见关系由关系图元决定。在无表图元而使用表定义的布局中，可为已解析且两端均可见的逻辑关系建立待路由边。路由为确定性正交路径，可绕行表矩形，处理共端点、关系交叉及自环。端点无法解析或在搜索范围内无避障路径时，边保留来源信息和空显示路径。关系基数仍以 PDM 为准。

## ADR-006 原生 ER 画布的图数据与绘制边界

- 日期：2026-11-21
- 状态：已采纳，Phase 6
- PDM 经 `BuildDiagramGraph` 和配套 BVH 生成场景。图投影补充表编码、字段原始类型及有来源依据的 PK/FK 标记。Renderer 只使用图场景、索引、相机和交互状态。通过相机用 `float64` 计算世界坐标到窗口 DIP 的位置，再交给 MyGo。字体、线宽和端点标记使用 DIP，画板布局继续保留原始坐标。
- 关系仍使用有序原始或回退路径，先绘制关系再覆盖表节点。箭头只用于逻辑外键方向与当前图元两端一致的已解析关系。关系基数由源模型提供。
- LOD 使用典型表宽的投影尺寸。以正宽度中位数除以 236 归一化相机比例，沿用远中近三档回滞。预排版前 64 表，后续按视口及选中对象准备文字。每画板最多保留 256 个表视觉和 512 个共享文字条目，按加入顺序淘汰，优先保留选中和当前可见表。超出容量的表暂时只画轮廓。每条文字预览限 256 个 Unicode 码点，原文保留在模型中。背景复用一块按 DPI 和主题更新的 32～256 像素网格纹理。绘制回调查询可见项并消费已准备的文字。切换画板时替换渲染器，释放旧缓存。
- 原生应用通过本地文件入口在后台解析和准备图数据。选择默认或用户指定画板时使用配套场景、索引与相机，默认适应当前画板。画板切换和视图复位清除选中与悬停；「显示全部」保持当前选中对象。自动布局状态、文件操作与画布工具复用首版双语资源。
- 最终运行原生画布基准，沿用固定种子、规模、疏密布局、窗口与操作周期。实测记录注明运行的是正式绘制链路，位置单位标定与 PowerDesigner 应用内对照仍按 P4-03 跟踪。
