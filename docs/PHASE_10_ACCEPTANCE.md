# Phase 10 跨平台与发布验收

验收日期为 2026-11-21（Asia/Shanghai）。原生界面测试使用合成夹具。真实 PDM 只在本机读取，P4-03 的 PowerDesigner 应用内对照继续为 `BLOCKED`。

## P10-01 macOS Apple Silicon

测试环境为 Apple M4、16 GiB、macOS 15.8.1（24H32）、Metal 3 和 60 Hz 显示器。显示器原生分辨率 5120×2880，工作区 2560×1440 DIP。构建使用 Go 1.27.4、MyGo v0.10.0、`CGO_ENABLED=0`。

### 应用包

`make package-macos VERSION=0.1.0` 通过，生成 `.out/release/0.1.0/macos-arm64/PdmView.app` 和同目录的 `PdmView-0.1.0-macos-arm64.zip`。重复构建使用独立目录。空值、非三段数字版本和含换行的版本均被拒绝。

`file` 和 `lipo` 确认 `arm64`。`plutil -lint` 通过，`Info.plist` 和 `xcrun vtool -show-build` 的最低系统版本均为 13.0。`codesign --verify --deep --strict` 通过，签名是 ad hoc。归档通过 `unzip -t` 和文件清单校验，只有应用包及签名元数据。该次归档的 SHA-256 为 `135cdcd5f6e39ab0746b512d189356c0ab451dd0343b4bf6220c98aca5efa6e6`。P10-03 加入许可证后的候选见发布记录。

### 运行时依赖

PdmView 主窗口通过 `WindowOptions.Content` 使用 MyGo 原生控件。原生画布探针检测到 `Page()==nil`、GPU 绘制和 2× 比例。正式包的 `otool -L` 未列出 WebKit、Electron 或 Node，进程采样时没有子进程。`vmmap` 在正式包进程中观察到 AppKit、Metal 以及系统 WebKit 映射。核对 MyGo v0.10.0 源码后确认，`internal/darwin/objc.go` 的 `load` 会在启动时调用 `mustDlopen` 加载系统 WebKit。PdmView 的界面没有创建 WebView。Gate 5 若要求进程完全不依赖 WebKit，当前 macOS 构建无法满足该项。

### 原生画布与内存

以正式 `renderer.Canvas` 运行 500 表、1,000 关系的密集场景，种子 `20261121`，窗口 1200×800 DIP。首次长测已显示原生 Metal 窗口，报告 `native=true page_nil=true gpu=true hidpi=true scale=2.00`。预热 5 秒后开始采样，180 秒场景未收到完成帧，200 秒 watchdog 退出，峰值 RSS 146.8 MiB。该次长测没有 FPS 结果，不能作为持续性能验收。日志和 RSS 样本位于 `.out/phase10/evidence/mac-500-dense-soak.log` 和 `mac-500-dense-rss.tsv`。

### 检查命令

- `make check` 通过，覆盖格式、11 个 Go 包的测试和 vet。
- `CGO_ENABLED=0 go test -mod=readonly -count=1 -tags e2e ./...` 通过。
- `go mod verify` 通过。
- `shasum -a 256 -c .local/phase3/sample.sha256` 通过。
