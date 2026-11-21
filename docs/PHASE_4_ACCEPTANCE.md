# Phase 4 领域模型与原始布局验收

2026-11-21 在 Apple M4、macOS 15.8.1、Go 1.27.4 上验证。领域模型和图映射契约见 [ADR-004](DECISIONS.md#adr-004-冻结-pdm-领域模型和-diagram-映射契约)。

## 本地样本验收

用 Go 标准库 `encoding/xml` 独立读取源 XML，再与 `pdm.Parse` 和 `graph.MapDiagram` 的最终输出比较。位置以原始矩形第一角表示，宽高取两角的差。核对覆盖图元顺序、图元及表 ID、两角、位置及宽高，以及每个表图元内的字段展示顺序。

| 检查 | 结果 |
|---|---|
| 画板 | 1 个，ID 匹配，使用原始布局 |
| 表图元 | 8/8 的身份、X/Y、宽度及高度与原始 XML 完全一致 |
| 字段 | 90/90 个画板对应字段按各表 XML 顺序进入图节点 |
| 未绘制的表 | 13 张模型表中有 5 张未放入该画板，映射保留这一区别 |
| 关系路径 | 样本含 0 个关系图元和 0 个路径点 |
| 源样本诊断 | 0 条 |

逐图元数值记录在 Git 忽略的 `.local/phase4/yian-layout.log`。提交中只保留统计与核对方法。

## 关系路径验收

用 Phase 3 留存的三份公开 XML 导出检查关系图元。逐条对照逻辑关系 ID、本画板源和目标图元、矩形及路径点次序。所有对照通过，世界坐标中的路径点与对应 XML 整数值一致。

| 公开导出 | 画板 | 表图元 | 图节点字段数 | 关系图元 | 有序路径点 |
|---|---:|---:|---:|---:|---:|
| 12.5.0.2169 | 1 | 4 | 17 | 2 | 4 |
| 15.1.0.2850 | 5 | 25 | 213 | 16 | 34 |
| 16.5.0.3982 | 3 | 8 | 100 | 5 | 18 |
| 合计 | 9 | 37 | 330 | 23 | 56 |

15.1 样本包含 3 个指向外部 Shortcut 的表图元，位置及关系端点全部保留。三份导出均未含显式 `ControlPoints`，控制点与折线路径的独立性由合成用例验证。公开导出来源见 [PDM 格式研究](PDM_FORMAT.md#未覆盖结构与版本范围)。

## PowerDesigner 应用内对照

**待完成。** 从 macOS 应用目录、Spotlight 和运行进程中未发现可用的本地 PowerDesigner 客户端或运行中的本地 Windows 环境。SAP 桌面客户端的环境要求见 [SAP 安装指南](https://help.sap.com/docs/SAP_POWERDESIGNER/90f79799eeeb4a50a938f82bce392697/c7d9c9de6e1b10149c65de17d779426e.html)。

XML 对照已通过。PowerDesigner 中的表位置、尺寸、实际字段显示和折线路径仍需本地 Windows 客户端核对。关系对照可用上表任一含路径的导出文件补测。坐标原始单位与 DIP 的换算及应用内视觉对照仍未验收。P4-03 保留待验收状态，Gate 1 和 Gate 2 的应用内布局对照项仍开放。映射结果提供 `AutoLayout`，本地化状态显示待接入正式画布。

## 回归验证

- DeepSeek 实现 P4-02，主 Agent 审查、补充同一画板中同表多图元的端点断言并集成。合成回归覆盖模型及 Package 作用域、同表不同图元、缺失或歧义端点、坐标与字段顺序、确定性回退和原始数据副本。
- 有效布局直接保留在世界坐标。缺失几何的节点使用四列网格，卡片基线为 236 × 144 个世界坐标单位。原始几何副本使用 `int64`，超出 `float64` 整数精确表示范围的坐标在场景投影中可能舍入。
- `CGO_ENABLED=0 go test -count=1 ./internal/graph ./internal/pdm`、两包 `go vet`、`make check` 及 `go vet -tags pdm_layout_check ./internal/graph` 均通过。单独运行四份样本的 XML 对照均通过。
- 原样本 SHA-256 与 Phase 3 一致。Git 忽略规则已核实，样本及逐图元日志留在本机。

## 本地复现

```sh
mkdir -p .local/phase4
PDM_LAYOUT_SAMPLE="$PWD/pdm_file/yian.pdm" PDM_LAYOUT_DETAILS=1 CGO_ENABLED=0 \
  go test -tags pdm_layout_check -run '^TestDiagramXMLSample$' -count=1 -v ./internal/graph \
  > .local/phase4/yian-layout.log
shasum -a 256 -c .local/phase3/sample.sha256
```

把 `PDM_LAYOUT_SAMPLE` 指向 `.local/phase3/public/` 中的本地公开导出，可以复现关系路径检查。详细输出包含图元 ID 和坐标，应重定向至 Git 忽略的本地报告。
