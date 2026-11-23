# PdmView 开发

## 工具链

- Go 模块最低版本为 1.27.3，验收工具链为 1.27.4。MyGo 固定为 v0.10.0，构建使用 `CGO_ENABLED=0`。
- macOS 打包在 macOS 上执行，需要 Xcode 命令行工具以及 `plutil`、`codesign`、`ditto` 和 `unzip`。Go 1.27 生成的 macOS 可执行文件最低版本为 macOS 13.0。
- Windows x64 支持交叉构建。打包脚本使用 Python 3 校验 EXE 中的产品图标。启动、图标显示、Direct3D 11、系统 DPI、文件面板和剪贴板在 Windows x64 桌面上验收。
- Lobe MyGo 在 Phase 0 使用 `v0.0.0-20261116124832-d88b4ad7f238` 验证过隔离示例。正式应用使用 MyGo 原生控件。参见 [API 验证记录](docs/MYGO_API_VERIFICATION.md)。

## 启动与测试

在仓库根目录运行。

```sh
go version
make env
CGO_ENABLED=0 go run ./cmd/pdmview
CGO_ENABLED=0 go run ./cmd/pdmview /path/to/model.pdm
make check
CGO_ENABLED=0 go test -mod=readonly -count=1 -tags e2e ./...
go mod verify
```

`make check` 检查格式并运行 `go test ./...` 和 `go vet ./...`。`-tags e2e` 启用用于原生自动化的调试入口。发布构建关闭 inspector，并在 Windows 上移除 WebView2 后端。

| 命令入口 | 用途 |
|---|---|
| `./cmd/pdmview` | 桌面应用 |
| `./cmd/pdm-inspect` | 本地 XML 解析统计。输出结构计数和诊断代码 |
| `./cmd/canvas-bench` | 合成画布基准。操作和采样参数见 [Phase 6 验收](docs/PHASE_6_ACCEPTANCE.md#复现) |
| `make probe` | Phase 0 原生技术探针 |

需要数据库执行复核时，使用 [Phase 9 的隔离 MySQL 8.0 流程](docs/PHASE_9_ACCEPTANCE.md#p9-04-mysql-80-实际执行)。仓库的 Golden Tests 使用合成 SQL，`make check` 无需数据库。

## 本地数据

真实 PDM 样本留在本地。`.pdm`、`.local/` 和 `.out/` 已被 Git 忽略。发布包从正式入口和依赖许可证文件按清单组装。桌面验收优先使用合成夹具，记录只引用真实样本的结构计数和校验结果。

发布命令、产物和签收步骤见 [发布记录](RELEASE.md)。
