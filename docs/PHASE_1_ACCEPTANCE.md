# Phase 1 验收记录

验收环境为 2026-11-21 的 Apple M4、macOS 15.8.1、Go 1.27.4、MyGo v0.10.0，使用 `CGO_ENABLED=0`。

## P1-01 原生最小窗口

```sh
CGO_ENABLED=0 go run ./cmd/pdmview
CGO_ENABLED=0 go test -count=1 ./cmd/pdmview
CGO_ENABLED=0 go vet ./cmd/pdmview
```

- `go test` 与 `go vet` 通过。测试核对标题、初始及最小尺寸、`WindowOptions.Content` 和窗口内容。
- `go build -tags e2e` 产物在桌面启动成功。MyGo 本地自动化列出一个可见的 `kind=content` 窗口，客户区为 1200×800 DIP，Metal 日志记录 GPU 绘制。进程快照无子进程，程序入口只创建 `ui.View` 原生窗口。
- macOS Accessibility 对本次测试进程请求 500×400 窗口，实际宽高限制为 800×600。通过系统关闭按钮退出，退出码 0。
- 实际截图为 2400×1600，已检查内容，保存在本地 `.out/phase1/p1-01.png`。自动化只在带 `e2e` 构建标签且设置了本地 socket 时启用。

## P1-04 双语资源、切换与偏好

- `go test -count=1 ./cmd/pdmview ./internal/app ./internal/i18n`、同范围 `go vet` 和 `make format-check` 通过。覆盖资源键与命名参数、英文回退、默认语言与已保存选择、无效配置、保留其他偏好和替换失败后的重试。
- 用独立测试用户目录启动，系统首选语言 `zh-Hans-CN`，缺省界面显示简体中文。通过 MyGo 原生菜单依次切换到英文、简体中文、英文，窗口文案即时更新。macOS Accessibility 从原生菜单读到当前项的选中标记。
- 用系统关闭按钮退出并重新启动。系统仍优先中文，程序读取 `PdmView/preferences.json` 中的 `language: en`，恢复英文和相应菜单标记。
- 在同一隔离目录内验证损坏 JSON。启动使用系统默认语言并提示偏好无法读取。选择英文后界面立即切换，显示英文保存失败提示，损坏文件内容原样保留。
- 实际截屏保存在 `.out/phase1/p1-04-en.png`。Windows 原生菜单和偏好行为列入 P10-02。

## P1-02 Painter 与 Canvas

```sh
mkdir -p .out/phase1
CGO_ENABLED=0 go build -o .out/phase1/painter ./cmd/mygo-probe
MYGO_FRAME_STATS=all .out/phase1/painter -smoke -smoke-out .out/phase1/p1-02.png
CGO_ENABLED=0 go test -count=1 ./cmd/mygo-probe
CGO_ENABLED=0 go vet ./cmd/mygo-probe
```

- 复用 Phase 0 探针。错开两张模拟表验证实际可见的虚线贝塞尔曲线。确认 `Fill`、`Stroke`、`Line`、`StrokePath`、`Clip`、`Transform`、`TextLayout`，技术证据见 [MyGo API 核对](MYGO_API_VERIFICATION.md)。
- 在 2× HiDPI 的 2800×1744 截图中观察到圆角节点、网格、曲线及中英数字混排。首帧日志确认硬件加速。用原生 automation 移动视口至 `(2500,2500)`，截图中节点和边消失，网格持续显示，再操作缩放至 118%。原图和裁剪图在 `.out/phase1/`。
- 目标测试与 vet 通过。新增断言检查曲线端点高度、移出视口后的绘制像素与可见计数，以及连续帧复用同一个 Canvas。Phase 0 的大规模基准仍使用原有线段和几何。
- `Draw` 随原生 UI 主线程的帧运行，同一帧可能调用多次。`Canvas.Draw` 使用本帧的 Painter 将网格绘入纹理，后续绘制复用 `Bitmap()`。文本排版在回调之前完成，Painter 仅在当前绘制回调内使用。

## P1-03 三栏原型

- 在 P1-04 验收通过后接入导航、图表画布和详情。画布用 MyGo Painter 绘制网格。两道原生分隔区保持独立尺寸比例，界面文案使用 P1-04 的资源。
- `go test -count=1 ./internal/ui ./cmd/pdmview` 通过。覆盖双语布局、两条分隔线拖动、键盘调整、语言刷新保留栏宽、最小宽度下的长警告。
- macOS 原生窗口实测将两条分隔线分别向右和向左移动 42 DIP。切换为英文后两个分隔位置仍相同。将窗口框缩至 800×600 DIP，在该尺寸验证中英文三栏标题和状态提示。内容截图为 1600×1144，窗口装饰占用 28 DIP 高度。原始截图为 `.out/phase1/p1-03-zh.png`、`p1-03-en-min.png` 与 `p1-03-zh-min.png`。

## 阶段检查

- `make check`、`CGO_ENABLED=0 go test -tags e2e -count=1 ./...` 和 `go mod verify` 通过。
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=readonly -o .out/phase1/pdmview-windows-amd64.exe ./cmd/pdmview` 通过。Windows 桌面验收留在 P10-02。
- Phase 1 的三栏画布用于布局验证。P2 的坐标与交互原型及性能任务尚未启动。
