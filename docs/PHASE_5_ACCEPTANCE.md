# Phase 5 图引擎验收

2026-11-21 在 Apple M4、macOS 15.8.1、Go 1.27.4 上执行 Go 图引擎测试与基准。契约见 [ADR-005](DECISIONS.md#adr-005-正式图引擎与关系路由)。

## P5-01 场景与来源

场景与几何类型从合成数据生成器移到正式图模块。Graph/PDM 定向测试覆盖画板、表和关系图元 ID 的逐项追溯，原始整数矩形、路径和控制点仍随来源记录保存。同一表的多个图元分别参与查询和命中。

## P5-02 坐标变换

本地 `yian.pdm` 的原始世界坐标适应 1024 × 768 DIP 视口，保留 24 DIP 边距。8/8 个表图元中心完成窗口、视口、世界坐标往返，屏幕误差均不超过 `1e-7` DIP。同样条件下原 Phase 2 相机的最小缩放比例会截断全图。合成测试另核对非零窗口原点、负世界坐标、宽跨度、缩放锚点和缩放限位。

## P5-03 空间索引

在本地样本上，8 次图元点查询及 1 次缩放视口查询与线性基准一致，场景换代后删除的节点不再出现。合成测试覆盖移动、增删、重排、边同步换代、数据隔离和并发读写；并发查询测试通过 Go race detector。换代在交互查询路径外完成。

以下为 500 节点、1,000 条边的确定性密集合成布局，Go 基准 3 次取中位数。局部及全图用例各包含一次节点查询和一次边查询。

| 操作 | 线性 | BVH |
|---|---:|---:|
| 单点查询 | 298.1 ns | 33.56 ns |
| 局部视口查询 | 8.034 µs | 1.650 µs |
| 全图查询 | 5.828 µs | 0.6999 µs |
| 全量重建并发布 | — | 2.11 ms |

局部及全图查询复用结果切片，每次 0 次分配。点查基准每次使用空结果切片，BVH 与线性各 1 次分配。一次索引重建分配约 1.24 MB、4,961 次分配。

## P5-04 命中测试

本地样本的 8 个表图元在适应比例的 0.5×、1×、2× 三档下合计 24/24 次命中，返回正确的画板、表和图元 ID。合成用例检查节点优先、同层逆绘制顺序、边真实线段、各缩放档的固定 DIP 容差、非零窗口偏移和画布外拒绝。

## P5-05 关系路径与自动路由

`BuildDiagramGraph` 先投影画板再处理缺失路径。原始路径、引用 ID 和独立控制点沿用 Phase 4 的记录。存在表图元的显式画板只绘制已有关系图元。表定义回退布局使用两端表均可解析的原有逻辑关系，方向为从表到主表。

路由器依次尝试直线、L 形和基于障碍通道的有界正交搜索。允许关系相交，线段不得穿越表内部。合成回归覆盖多重阻挡、窄间距、备选侧端口、共端点、自环、相交和被包含的不可达端点。每个搜索窗口最多处理 256 个障碍，两个坐标轴各最多 192 条位置线，每条边最多扩展 30,000 个位置。超限或没有安全路径时保留来源和空显示路径。

`yian.pdm` 含 0 条关系路径。三份公开导出的 **23 条原始关系、56 个路径点**全部保持。仅清除内存副本的路径后，23/23 条关系重新路由成功。独立校验确认两端落在对应表边界上，69 个生成线段均正交且未穿越表内部。

使用 Phase 2 的密集布局和固定 seed `20261121`，保留原有关系端点并清除全部显示路径。用独立校验逐段检查全部边。计时使用默认优化构建，运行 3 次取中位数，包括路由用的 BVH 构建和计算，不含 PDM 解析或绘制。

| 节点 | 成功路由 | 核对线段 | 用时中位数 | 累计分配/次 |
|---:|---:|---:|---:|---:|
| 50 | 100/100 | 512 | 1.879 ms | 2.58 MB |
| 150 | 300/300 | 1,526 | 12.33 ms | 15.50 MB |
| 300 | 600/600 | 3,105 | 48.19 ms | 56.57 MB |
| 500 | 1,000/1,000 | 5,092 | 106.29 ms | 123.43 MB |

分配量为 Go 基准的累计字节数。大型图的路由应在后台构建场景时执行，画布查询使用生成后的路径和空间索引。

## 验收命令

`make check`、图包 race 检查、带验收标签的 vet、四份本地 XML 导出对照及独立密集图核对均通过。私有样本校验值与 Phase 3 一致，样本和日志由 Git 忽略。P4-03 的 PowerDesigner 应用内对照仍为 `BLOCKED`。

```sh
make check
CGO_ENABLED=1 go test -race -count=1 ./internal/graph
CGO_ENABLED=0 go vet -tags pdm_layout_check ./internal/graph
PDM_LAYOUT_SAMPLE="$PWD/pdm_file/yian.pdm" PDM_LAYOUT_DETAILS=0 CGO_ENABLED=0 \
  go test -tags pdm_layout_check -run '^(TestDiagramXMLSample|TestPhase5Sample.*)$' -count=1 -v ./internal/graph
CGO_ENABLED=0 go test -tags pdm_layout_check -run '^TestPhase5DenseRoutes$' -count=1 -v ./internal/graph
CGO_ENABLED=0 go test ./internal/graph -run '^$' -bench '^BenchmarkSpatial' -benchmem -benchtime=300ms -count=3
CGO_ENABLED=0 go test ./internal/graph -run '^$' -bench '^BenchmarkRouteMissingEdges$' -benchmem -benchtime=500ms -count=3
```

公开导出用本地 `.local/phase3/public/pd-12.5.xml`、`pd-15.1.xml` 和 `pd-16.5.xml` 逐一替换 `PDM_LAYOUT_SAMPLE`。样本日志保存在 `.local/phase5/`，基准日志保存在 `.out/phase5/`。
