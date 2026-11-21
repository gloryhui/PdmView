# Phase 10 跨平台与发布验收

验收日期为 2026-11-21（Asia/Shanghai）。原生界面测试使用合成夹具。真实 PDM 只在本机读取，P4-03 的 PowerDesigner 应用内对照继续为 `BLOCKED`。

## P10-01 macOS Apple Silicon

测试环境为 Apple M4、16 GiB、macOS 15.8.1（24H32）、Metal 3 和 60 Hz 显示器。显示器原生分辨率 5120×2880，工作区 2560×1440 DIP。构建使用 Go 1.27.4、MyGo v0.10.0、`CGO_ENABLED=0`。

### 应用包

`make package-macos VERSION=0.1.0` 通过，生成 `.out/release/0.1.0/macos-arm64/PdmView.app` 和同目录的 `PdmView-0.1.0-macos-arm64.zip`。重复构建使用独立目录。空值、非三段数字版本和含换行的版本均被拒绝。

`file` 和 `lipo` 确认 `arm64`。`plutil -lint` 通过，`Info.plist` 和 `xcrun vtool -show-build` 的最低系统版本均为 13.0。`codesign --verify --deep --strict` 通过，签名是 ad hoc。归档通过 `unzip -t` 和文件清单校验，只有应用包及签名元数据。该次归档的 SHA-256 为 `135cdcd5f6e39ab0746b512d189356c0ab451dd0343b4bf6220c98aca5efa6e6`。P10-03 加入许可证后的候选见发布记录。

### 原生界面和系统服务

| 检查 | 实测结果 |
|---|---|
| 启动和退出 | 通过 LaunchServices 启动正式 `.app`，关闭窗口后进程退出。修正最低系统版本后的包再次启动、恢复偏好并正常退出。 |
| 双语切换 | 系统首选 `zh-Hans-CN`，隔离目录没有偏好时默认简体中文。原生菜单双向即时切换并显示当前语言的勾选。保存 `zh-CN` 后退出，重启恢复中文。应用配置目录通过测试进程的 `HOME` 隔离。 |
| 原生打开、详情与搜索 | 打开面板可取消。选中合成 PDM 后显示画板和详情，搜索 `alpha` 定位到两条匹配。打开损坏的 PDM 显示中文应用错误提示，并保留当前合成模型及选中表。 |
| Retina 与最小窗口 | 请求 500×400 后系统限制为 800×600 点，观察到 1600×1200 像素窗口。画板和详情的中文及拉丁文字均可辨认，详情滚轮生效。最小窗口的英文详情中，长连续注释有右侧截断。此次未实测物理触控板捏合。 |
| 原生 SQL 保存 | 面板建议名 `phase8_fixture.sql`，支持取消。中英文各保存一份 1,445 字节文件，逐字节与 Phase 9 合成基准一致。对本次合成文件再次保存时显示系统替换确认，确认后的文件仍与基准一致。 |
| 系统剪贴板 | 正式包的中文复制显示成功，随后剪贴板版本变化中止了字节对照，未确认当次恢复。重新构建同源的 `e2e` 原生程序并复核，两种语言的完整表信息 3,058 字节、全库 SQL 1,445 字节、单表 SQL 944 字节分别逐字节一致。全库 SQL 与合成基准文件一致。两轮均确认原剪贴板恢复。 |
| 测试覆盖界限 | 正式桌面流程使用合成夹具，真实样本仅通过本地摘要和校验和复核。 |

界面文案检查覆盖菜单与工具标签、搜索、详情、预览、保存和错误反馈。错误提示保留，供打开失败时识别结果。正式包的两份合成导出保存在 `.out/phase10/evidence/mac-export-en/` 和 `mac-export-zh/`。两份的 SHA-256 均为 `b66f80149784f02376c887fc1baf23c483d983214e0fc9483c331de1b77f2fa1`。

### 运行时依赖

PdmView 主窗口通过 `WindowOptions.Content` 使用 MyGo 原生控件。原生画布探针检测到 `Page()==nil`、GPU 绘制和 2× 比例。正式包的 `otool -L` 未列出 WebKit、Electron 或 Node，进程采样时没有子进程。`vmmap` 在正式包进程中观察到 AppKit、Metal 以及系统 WebKit 映射。核对 MyGo v0.10.0 源码后确认，`internal/darwin/objc.go` 的 `load` 会在启动时调用 `mustDlopen` 加载系统 WebKit。PdmView 的界面没有创建 WebView。Gate 5 若要求进程完全不依赖 WebKit，当前 macOS 构建无法满足该项。

补查了 [MyGo v0.10.0](https://github.com/egoist/mygo/releases/tag/v0.10.0)、[官方主分支的 Darwin 初始化](https://github.com/egoist/mygo/blob/main/internal/darwin/objc.go)及 [原生窗口构建标签说明](https://mygo.run/docs/ui/windows#no-webview)。锁定版本与官方主分支均直接加载系统 WebKit。用 `go list` 比对，Darwin 在启用 `mygo_nowebview` 前后选中的源文件完全相同。

在项目忽略的 `.out/phase10/followup/webkit-overlay.d4ffM1/` 下复制 MyGo v0.10.0，使用独立 `-modfile` 与 Go `-overlay`。实验副本去掉 WebKit 的启动 `dlopen`、`WKWebView` 子类和 scheme handler 的启动注册。该副本仅用于原生验证。`cmd/pdmview`、`internal/app`、`internal/ui` 定向测试通过。启动合成夹具后，原生 SQL 预览可双语切换，1,445 字节内容保持一致，并正常退出。

| 运行对照 | dyld 实测结果 |
|---|---|
| 锁定的 MyGo | 直接 `dlopen` 系统 `WebKit.framework/WebKit` |
| 隔离实验 | 上述直接调用消失。AppKit 在启动时加载 `WritingToolsUI`，其弱依赖 `GenerativeAssistantSettings` 又引入 WebKit。`vmmap` 仍显示 WebKit、WebCore、WebKitLegacy、AppKit 与 Metal。 |

可在 MyGo 上游将框架和对应 Objective-C 类注册移到 Web 功能首次使用时，以消除框架自身的直接加载。该实验同时表明，延迟 MyGo 的加载不足以保证 macOS 进程没有 WebKit 映射。Apple 对 Writing Tools 的公开 API 调整的是具体视图的编辑行为。[Apple 的 API 说明](https://developer.apple.com/documentation/appkit/writing-tools)。目前未找到官方公开开关能阻止实测的 AppKit 加载链。实验改动留在本地，`go.mod`、`go.sum` 和发布包未变。

dyld 日志位于 `.out/phase10/followup/loader-fixed-dyld.log` 和 `loader-overlay-dyld.log`，隔离版 UI 与 `vmmap` 证据位于实验目录。

### 原生画布与内存

以正式 `renderer.Canvas` 运行 500 表、1,000 关系的密集场景，种子 `20261121`，窗口 1200×800 DIP。首次长测已显示原生 Metal 窗口，报告 `native=true page_nil=true gpu=true hidpi=true scale=2.00`。预热 5 秒后开始采样，180 秒场景未收到完成帧，200 秒 watchdog 退出，峰值 RSS 146.8 MiB。该次长测没有 FPS 结果，不能作为持续性能验收。日志和 RSS 样本位于 `.out/phase10/evidence/mac-500-dense-soak.log` 和 `mac-500-dense-rss.tsv`。

启用逐帧日志并保持显示唤醒后复跑，日志记录到原生 GPU 绘制，本次同样由 200 秒 watchdog 结束，峰值 RSS 147.0 MiB。单独做 10 秒烟测也由 27 秒 watchdog 结束。帧调度停止的原因尚未确认。复跑和烟测日志位于 `.out/phase10/evidence/mac-500-dense-retry.log` 和 `mac-500-dense-smoke.log`。此前已完成的短时性能数据见 [Phase 6 正式渲染器基准](PHASE_6_ACCEPTANCE.md#正式渲染器性能)。

### 长测超时复查

在同一台 Mac 重新编译原基准入口。500 表密集图的 10 秒对照完成 598 帧，实测 59.79 FPS，帧间隔 P95 17.515 ms。随后不置顶的 180 秒对照再次触发 200 秒 watchdog，全程平均 CPU 用时不足单核的 1%，峰值 RSS 135.6 MiB。停帧时用 `sample` 查询基准进程，主线程在 `NSApplication` 事件循环的 `mach_msg` 中等待。测试窗口仍存在且未最小化，当时在其他窗口之后。MyGo 的 `holdFrame` 会在窗口被系统判为不可见时暂停绘制，显示后恢复。证据保存在 `.out/phase10/followup/baseline-10s.log`、`baseline-180s.log`、`baseline-180s-rss.tsv` 和 `stalled-sample.txt`。

基准增加可选 `-always-on-top`，只影响基准窗口。watchdog 通过 MyGo 的 `Context.WindowHidden` 和最后一帧时间输出诊断。渲染器、显示节奏与既有测量口径保持一致。修改后不置顶的 10 秒对照完成 600 帧、60.00 FPS。置顶完成连续 180 秒采样，所有帧均由 Metal GPU 提交。

受控对照在采样开始后最小化基准窗口，22 秒 watchdog 记录 `window_hidden=true`、1 次隐藏状态变化、130 个绘制回调和 10 个采样帧，最后一帧已过去 19.262 秒。这复现了系统隐藏窗口后暂停绘制所引发的同类超时。原有超时发生时尚未记录遮挡事件，不能逐次归因。

| 场景 | 帧数 | FPS | 帧间隔 P95 | MyGo 帧 P95 | CPU（单核 100%） | 峰值 RSS |
|---|---:|---:|---:|---:|---:|---:|
| 500 表 / 1,000 关系密集图，1200×800 DIP，2×，预热 5 秒，采样 180 秒 | 10,775 | 59.86 | 17.608 ms | 3.9 ms | 20.4% | 142.9 MiB |

可见表和关系达到 500 / 1,000，三档 LOD 均触发，网格缓存命中 10,775/10,775，未准备可见表为 0。每 8 秒取一次 RSS，共 23 点，采样范围 93.8–138.0 MiB，前半与后半中位数分别为 137.2 和 131.2 MiB。该次 3 分钟工况未见持续内存增长。

```sh
mkdir -p .out/phase10/followup
CGO_ENABLED=0 go build -mod=readonly -trimpath -o .out/phase10/followup/canvas-bench-visible ./cmd/canvas-bench
caffeinate -diu /usr/bin/time -lp env -u MYGO_UNCAPPED MYGO_FRAME_STATS=all \
  .out/phase10/followup/canvas-bench-visible -nodes 500 -density dense -seed 20261121 \
  -always-on-top -warmup 5s -sample 180s
```

原始数据在 `.out/phase10/followup/visible-180s.log`、`visible-180s-rss.tsv`、`non-top-occlusion.log` 和 `minimized-occlusion-retry.log`。

### 检查命令

- `make check` 通过，覆盖格式、11 个 Go 包的测试和 vet。
- `CGO_ENABLED=0 go test -mod=readonly -count=1 -tags e2e ./...` 通过。
- `go mod verify` 通过。
- `shasum -a 256 -c .local/phase3/sample.sha256` 通过。

## P10-02 Windows x64

`make package-windows VERSION=0.1.0` 在 macOS 上交叉构建正式入口。`file` 和 `xcrun llvm-objdump -p` 确认 PE32+、AMD64、Windows GUI 子系统。`go version -m` 确认 `GOOS=windows`、`GOARCH=amd64`、`CGO_ENABLED=0` 及 `mygo_noinspector,mygo_nowebview`。已通过 WebView2 标记、ZIP 完整性和可执行文件 SHA-256 核对。

按用户安排，Windows 桌面验收延期，由后续 Windows 环境执行。启动与退出、D3D11、双语与偏好恢复、DPI 和中文字体、鼠标滚轮、大图性能、系统剪贴板、原生打开和保存及文件权限提示均待实机验证。交接步骤见 [发布记录](../RELEASE.md#windows-本机验收交接)。

## P10-03 文档与发布检查

- 核对 [README](../README.md)、[开发说明](../DEVELOPMENT.md) 与 [发布记录](../RELEASE.md) 的本地链接、启动入口、PDM 与 MySQL 支持范围及已知限制。README 截图取自 Phase 7 的原生合成夹具验收。
- 复建两个候选归档。Mac 归档含 11 个许可证或专利原文文件，Windows 含 13 个，另各附一份 `modules.txt`。已独立比对每个原文的归档内 SHA-256、完整文件清单和归档完整性，无 PDM、测试夹具或日志。
- 重新验证最终 Mac 应用签名。通过 LaunchServices 启动含许可证的最终 `.app`，确认运行路径、隔离偏好的中文恢复和正常退出。最终 Windows EXE 的 SHA-256 与已核对的交叉构建一致。
- `sh -n`、`make check`、e2e 标签 Go 测试、`go mod verify` 和 Windows x64 发布标签交叉 `go vet` 通过。本地真实样本 SHA-256 仍与 Phase 3 一致。

发布文档和本地候选检查完成。macOS 画布的 180 秒工况通过。物理触控板、macOS WebKit 启动加载与延期的 Windows 桌面验收仍影响 Gate 5。
