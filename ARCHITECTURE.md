# 架构与模块边界

## 1. 总体数据流

```text
PDM File
  |
  v
XML Reader / Version Detector
  |
  v
Raw PDM Representation + Diagnostics
  |
  v
PDM Domain Model
  |-----------------------------|
  v                             v
Diagram / Graph Model        MySQL SQL Generator
  |                             |
  v                             v
Graph Layout / Spatial Index  SQL Preview / Export
  |
  v
MyGo Canvas Renderer
  |
  v
Native UI
```

UI 只负责交互、状态展示和调用应用服务。不得直接解析 XML、拼接 SQL 或实现图布局。

## 2. 建议模块

### `internal/pdm`
职责：PDM 文件读取、版本识别、XML 对象解析、对象引用解析、统一 Domain Model、诊断信息。

约束：
- 保留原始 ID，引用关系通过 ID 解析。
- 区分 Name 与 Code。
- 保留字段顺序、NULL 与空值、默认值原始语义。
- 未识别节点/属性产生诊断；必要时保留原始片段。
- 解析器不依赖 UI、Graph Renderer、SQL Generator。

### `internal/graph`
职责：将 Diagram Symbol 与模型对象映射为图节点和边；坐标变换、视口、空间索引、命中测试、布局备用方案。

约束：
- 原始 Diagram 几何信息优先。
- 图布局使用独立世界坐标。
- 自动布局只作为缺失布局时的 fallback。
- 不依赖 MyGo Painter。

### `internal/renderer`
职责：把 Graph Model 绘制到 MyGo Canvas/Painter。

约束：
- 只消费预先计算的图形数据。
- Draw 回调不得解析 XML、执行自动布局、重建空间索引或进行昂贵文本测量。
- 支持视口裁剪、LOD、缓存和可见关系线筛选。
- 字体、HiDPI、文字测量必须在真实设备验证。

### `internal/sqlgen`
职责：MySQL DDL 生成、标识符转义、类型映射、依赖分析、诊断。

约束：
- 单表 SQL 和全库 SQL 使用同一套生成器。
- 表先创建，外键可在所有表创建后通过 ALTER TABLE 添加，以处理循环依赖。
- 模型级生成接收 PDM 文档，递归读取 Package，按物理表名和外键名稳定排序。Package 是组织结构，显式库或 Schema 限定需要先确认映射。
- 阻断诊断导致整个生成请求返回空 SQL。库名、字符集及排序规则可由调用方明确指定，缺少已确认的字符集或排序规则来源时沿用服务端默认值。
- 未知类型/不支持属性必须生成警告或阻断错误，不能静默替换。
- SQL 输出稳定、可重复，测试采用 golden files + MySQL 集成测试。

### `internal/app`
职责：应用生命周期、打开文件流程、当前模型状态、异步任务协调。

### `internal/ui`
职责：主窗口、模型树、画布、详情、SQL 预览、搜索、工具栏。

### 界面国际化与偏好

- Phase 1 为正式应用建立统一的文案查询入口。随程序内置 `zh-CN`、`en` 两套资源，共用稳定的消息键和命名参数。单条简中文案缺失时回退到英文，测试检查两套资源键与参数一致。
- 应用层持有当前语言并负责偏好读写。启动时在构建第一个窗口前决定生效语言。切换时在 UI 线程刷新原生菜单和当前界面文案，保留已加载的应用状态。画布中的应用文案在绘制前重新排版，模型原文沿用既有数据。
- 偏好保存到 `os.UserConfigDir()` 返回目录下的 `PdmView/preferences.json`，字段 `language` 仅接受 `zh-CN` 或 `en`。更新时保留其他偏好项，写入失败时保留上一次已成功保存的偏好，并在当前语言下报告失败。
- 解析器和 SQL 生成器向上提供诊断标识与格式化参数，界面负责生成用户提示。Graph、PDM 解析、诊断标识和 SQL 输出均独立于界面语言。

## 3. 领域模型契约

Phase 4 冻结 `internal/pdm` 中的领域模型。对象归属、缺失值语义、画板图元身份、原始几何及映射接口见 [ADR-004](docs/DECISIONS.md#adr-004-冻结-pdm-领域模型和-diagram-映射契约)。

映射层保留原始坐标及单位，通过世界坐标交付给 Graph。原始单位到实际显示尺寸的标定以 PowerDesigner 应用对照为准。

## 4. 并发与 UI 线程

- 文件解析在后台执行。
- 大型布局计算在后台执行。
- SQL 生成在后台执行。
- UI 线程只接收结果并更新状态。
- 所有跨线程 UI 更新必须遵守 MyGo 官方线程/事件模型。
- 取消旧任务，避免用户连续打开文件时旧结果覆盖新结果。

## 5. 依赖与版本

- 调研时读取 MyGo 官方文档和仓库当前版本。
- 记录 Go、MyGo、Lobe MyGo 的实际版本与兼容性。
- 不要未经验证假设 `go 1.27+` 一定可用；以当前官方稳定版和项目依赖实际要求为准。
- 不得编造 Painter、Canvas、剪贴板、文件选择器 API。先编译运行最小示例。

## 6. Phase 0 骨架与错误约定

`cmd/mygo-probe` 是独立技术验证入口，`scripts` 放可重复执行的验证命令，`docs` 记录架构决策和实际证据，`tasks` 记录施工和验收。正式应用及 `internal` 下的包随各自阶段落地。

- 命令参数无效时输出到 stderr 并返回非零退出码。窗口启动失败时记录操作和错误原因。
- 对文件、解析、绘制和导出返回的错误，在所属边界补充操作上下文，保留原始错误链。应用层负责把可恢复的错误转成用户状态。
- 日志写 stderr。常规诊断记录动作、平台或对象计数。真实 PDM 内容、文件路径、SQL 正文和剪贴板内容只在明确的本地交互中使用，常规诊断不打印。
