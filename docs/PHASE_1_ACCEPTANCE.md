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
