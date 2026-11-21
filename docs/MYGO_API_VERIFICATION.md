# MyGo Native UI 技术验证

## 版本和官方证据

| 项目 | 核对结果 |
|---|---|
| Go | 系统默认 1.25.0，使用 Go 1.27.4 构建和测试。2026-11-21 查阅的官方 1.27 系列稳定补丁版为 1.27.4。 |
| MyGo | `v0.10.0`，官方 tag `ebde9399baf2734b3361856a53f4036859973d1d`，模块要求 Go 1.27.3。 |
| Lobe MyGo | `main` 没有发行 tag，解析为 `v0.0.0-20261116124832-d88b4ad7f238`，模块要求 Go 1.27.3，声明依赖 MyGo `v0.8.4-0.20261108082913-9233663736ca`。 |

来源：[Go 版本历史](https://go.dev/doc/devel/release#go1.27.0)、[MyGo Release](https://github.com/egoist/mygo/releases/tag/v0.10.0)、[MyGo go.mod](https://github.com/egoist/mygo/blob/v0.10.0/go.mod)、[Lobe MyGo go.mod](https://github.com/lobehub/lobe-mygo/blob/d88b4ad7f238950ed04a3be6a82501540330dbd6/go.mod)。

## API 源码核对

以下均以 v0.10.0 tag 为准。实跑证据见同文的验收记录。

| 能力 | 核对到的接口 | 官方证据 |
|---|---|---|
| 原生窗口与线程 | `App.WhenReady`、`App.Run`、`NewWindow`、`WindowOptions.Content: ui.View(...)`、`RunOnMain`、`Window.Invalidate` | [最小官方示例](https://github.com/egoist/mygo/blob/v0.10.0/examples/counter-native/main.go)、[框架线程与窗口架构](https://github.com/egoist/mygo/blob/v0.10.0/docs/architecture.md) |
| Painter | `Element.Draw` 回调获得 `*ui.Painter` 和 DIP 矩形。实测 `Fill`、`Stroke`、`Clip`、`Transform`、`Visible`。`Scaling(...).Then(Translation(...))` 依次应用变换。 | [绘制指南](https://github.com/egoist/mygo/blob/v0.10.0/docs/ui/drawing.md)、[Painter 实现](https://github.com/egoist/mygo/blob/v0.10.0/ui/paint.go) |
| Canvas | `ui.NewCanvas(w,h)` 以像素指定离屏面，`Canvas.Draw` 生成内容，`Canvas.Bitmap` 取得供合成的位图。 | [Canvas 源码](https://github.com/egoist/mygo/blob/v0.10.0/ui/canvas.go) |
| Path | `MoveTo`、`LineTo`、`CubeTo`、`Dash`、`Contains`，以及 `Painter.StrokePath`、`Painter.ClipPath`。 | [Path 源码](https://github.com/egoist/mygo/blob/v0.10.0/ui/path.go)、[Path 裁剪](https://github.com/egoist/mygo/blob/v0.10.0/ui/mask.go) |
| 文字 | `ui.ShapeText` 创建排版，`Painter.TextLayout` 消费排版，`Painter.Scale` 返回每个 DIP 的物理像素数。 | [文字排版](https://github.com/egoist/mygo/blob/v0.10.0/ui/text_layout.go)、[DPI 接口](https://github.com/egoist/mygo/blob/v0.10.0/ui/glyphs.go) |
| 指针与命中 | 实测 `Element.HandleInput` 的 `InputPointerDown/Move/Up`、`InputScroll` 和取消事件。`InputEvent.X/Y` 为元素局部 DIP，探针显式反算世界坐标，用 `Path.Contains` 命中表节点。 | [输入指南](https://github.com/egoist/mygo/blob/v0.10.0/docs/ui/input.md)、[事件坐标和取消语义](https://github.com/egoist/mygo/blob/v0.10.0/ui/handler.go) |
| 原生文件选择 | `mygo.Dialog.Open`、`OpenDialogOptions.Parent`、`FileFilter.Extensions`。取消返回空路径列表，macOS 后端调用 `NSOpenPanel`。 | [Dialog API](https://github.com/egoist/mygo/blob/v0.10.0/dialog.go)、[macOS 后端](https://github.com/egoist/mygo/blob/v0.10.0/internal/darwin/dialog.go) |
| 系统剪贴板 | `mygo.Clipboard.Write`、`mygo.Clipboard.Read`、`mygo.Clipboard.ReadFormat`，使用 `transfer` 数据类型。 | [剪贴板 API](https://github.com/egoist/mygo/blob/v0.10.0/clipboard.go)、[官方示例](https://github.com/egoist/mygo/blob/v0.10.0/examples/clipboard/main.go) |
| 屏幕 DPI | `mygo.Screen.PrimaryDisplay().ScaleFactor` 可查询主屏，当前绘制面的像素比以 `Painter.Scale()` 为准。 | [显示器 API](https://github.com/egoist/mygo/blob/v0.10.0/modules.go)、[绘制 DPI 实现](https://github.com/egoist/mygo/blob/v0.10.0/ui/glyphs.go) |

## 平台条件

MyGo v0.10.0 的官方要求是 macOS 12+ 或 Windows 10/11 x64、arm64。原生桌面可用 `CGO_ENABLED=0` 构建，系统图形库在运行时加载。macOS 后端用 AppKit 和 Metal，Windows 后端用系统 Direct3D 11。打包签名和发布验证属于 Phase 10。

来源：[官方要求](https://github.com/egoist/mygo/blob/v0.10.0/docs/README.md#requirements)、[Metal](https://github.com/egoist/mygo/blob/v0.10.0/internal/gpu/metal/metal.go)、[D3D11](https://github.com/egoist/mygo/blob/v0.10.0/internal/gpu/d3d11/device/device.go)。

## Lobe MyGo 兼容检查

[Lobe MyGo 官方桌面示例](https://github.com/lobehub/lobe-mygo/blob/d88b4ad7f238950ed04a3be6a82501540330dbd6/examples/desktop/main.go) 直接使用 `mygo`、`ui` 和 Lobe Go 组件。官网上在浏览器中的预览是另一个 WebAssembly 构建入口。

在本地隔离模块同时固定上述两个依赖版本。`CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -mod=mod -o ../../.out/lobe-desktop github.com/lobehub/lobe-mygo/examples/desktop` 成功，`go list -m` 核实实际选用 MyGo `v0.10.0`。`CGO_ENABLED=0 go test -mod=mod github.com/lobehub/lobe-mygo/pkg/action github.com/lobehub/lobe-mygo/pkg/theme` 两个包均通过。桌面示例启动后首次绘制日志报告 GPU 提交。

## macOS 原生系统服务实测

在 Apple M4 桌面会话，独立本地验证程序通过 `WindowOptions.Content` 创建窗口。实际检查 `NativeHandle() != 0`、`Page() == nil`，首帧日志报告 GPU 提交。

原生窗口和 WebView 创建按上述探针结果验收。macOS 系统 WebKit 的启动加载见 [Phase 10 运行记录](PHASE_10_ACCEPTANCE.md#原生-ui-与-webkit-加载)。

- 调用 `Dialog.Open`，设置 `.pdm` 文件过滤。通过 MyGo 官方 macOS 测试钩子观察系统面板在屏幕上出现，触发取消，API 返回零个路径且无错误。自动发送方向键和回车的文件选择尝试未得到选中结果，选中文件返回路径仍需验证。
- 系统剪贴板写入并读回包含中文的合成文本，字节相同。正式探针通过本地 automation 实际触发按钮，状态显示 Unicode 与专用格式往返成功，原剪贴板内容已恢复。恢复前核对专用标记，极短时间内的外部并发写仍存在竞态。

## P0-02 原生探针验收

探针位于 `cmd/mygo-probe`。默认画两张模拟表和一条虚线关系，使用离屏 Canvas 重复合成网格，保留预排版的混合文字。矩形裁剪与变换在原生帧中运行。

- 本机 smoke 输出 `native=true frames=3 scale=2.00 accelerated=true`，进程退出码 0。`Window.Page()` 返回 `nil`，原生句柄非零。MyGo 帧日志报告 GPU 提交，macOS 此分支使用 Metal。
- 官方 automation 插件经本地 Unix socket 返回一个可见的 `kind=content` 窗口，无子进程。向运行中的窗口发送点击、拖动、滚轮和缩放动作。实际命中两张模拟表，空白点击未选中。拖动使相机到 `(-60,-25)`，滚轮后到 `(-50,5)`，缩放按钮到 118%，复位后第二张表命中。
- 窗口内容为 `1400×872` DIP，捕获为 `2800×1744` 像素。检查截图确认两张表、虚线关系、网格以及中英日和西里尔文字可辨识。截图是窗口内容捕获，原生 GPU 提交由帧日志另行核对。
- `ui.Tester` 覆盖绘制和 Canvas，点击与缩放后像素命中对齐，滚轮、拖动、取消事件，2× 缩放，按钮触发边界和选中路径隐私，以及三档数据规模。系统剪贴板和文件面板的实际调用另见原生系统服务实测。
- `CGO_ENABLED=0 go test -tags e2e -count=1 ./cmd/mygo-probe`、`go vet -tags e2e ./cmd/mygo-probe` 通过。
- Windows x64 使用 `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=readonly` 交叉构建通过，`file` 识别为 PE32+ x86-64。D3D11、Windows DPI 与系统服务仍需 Windows 桌面实跑。

## Gate 0 性能原型

macOS 15.8.1、Apple M4、16 GiB、5120×2880 等效 2560×1440 @ 60 Hz，Go 1.27.4，默认优化的 macOS arm64 构建，`CGO_ENABLED=0`，Metal、2×。采用窗口内容 `1400×872` DIP，启动后将固定合成场景缩放至全部可见，按显示刷新节奏连续重绘。每档预热 2 秒、采样 10 秒，顺序运行。

| 总节点/边 | 可见节点/边 | FPS | 帧间隔 P95 | Draw 回调 P95 | 进程 CPU | 峰值 RSS |
|---|---|---:|---:|---:|---:|---:|
| 100/200 | 100/200 | 59.90 | 17.526 ms | 2.391 ms | 17.6% | 99.0 MiB |
| 300/600 | 300/600 | 59.90 | 18.257 ms | 4.075 ms | 25.8% | 125.0 MiB |
| 500/1000 | 500/1000 | 60.00 | 17.477 ms | 4.393 ms | 29.7% | 155.2 MiB |

帧间隔取进入采样区间后连续绘制回调完成之间的时间，包含显示刷新等待。回调耗时不含 GPU 提交。进程 CPU 是 `/usr/bin/time -lp` 的 `(user+sys)/real`，含预热和退出，以单核为 100%。峰值 RSS 同样覆盖整次运行。

首轮以所有关系为虚线贝塞尔曲线，200 条为 1.40 FPS、峰值 RSS 约 1.48 GiB，600 条为 0.30 FPS、约 3.20 GiB。1000 条一档在采样完成前停止。MyGo 日志显示路径遮罩图集饱和并回退到 CPU。上表使用不透明 `Painter.StrokePolyline` 线段，官方 GPU 快速路径不需要整条线的遮罩。默认两表探针仍保留虚线贝塞尔验证。这项结果说明线型和渲染路径对性能的影响，真实 PDM 大画布继续按 Phase 2 和 Phase 6 的门禁验收。

## 复现

```sh
mkdir -p .out
CGO_ENABLED=0 go build -mod=readonly -o .out/mygo-probe ./cmd/mygo-probe
MYGO_FRAME_STATS=all .out/mygo-probe -smoke -smoke-out .out/mygo-smoke.png
for nodes in 100 300 500; do
  MYGO_FRAME_STATS=all /usr/bin/time -lp .out/mygo-probe \
    -bench -nodes "$nodes" -warmup 2s -measure 10s > ".out/bench-polyline-$nodes.log" 2>&1
done
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -mod=readonly -o .out/mygo-probe-windows-amd64.exe ./cmd/mygo-probe
```

运行 `make probe` 可手动验证画布和系统服务。自动化使用 `go build -tags e2e` 后设置独立的 `MYGO_AUTOMATION` Unix socket，由 [官方 automation 协议](https://github.com/egoist/mygo/blob/v0.10.0/docs/plugins/automation.md) 的 `windows`、`tree`、`input`、`act` 和 `capture` 查询与输入。常规构建不包含该插件。
