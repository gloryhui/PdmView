# 技术决策

## ADR-001 原生桌面与工具链

- 日期：2026-11-21
- 状态：已采纳
- 项目模块使用 `pdmview`。项目尚无远端或可分配的发布模块路径。
- MyGo 锁定官方 tag `v0.10.0`（`ebde9399baf2734b3361856a53f4036859973d1d`）。`go.mod` 声明依赖要求的 Go 1.27.3，验证使用工具链 Go 1.27.4。桌面构建以 `CGO_ENABLED=0` 为基准。
- 所有窗口使用 `WindowOptions.Content` 和 `ui.View`。macOS arm64 作为实际验证平台，Windows x64 单独记录交叉构建与真实运行结果。
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
