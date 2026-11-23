# PdmView 发布记录

## 1.0.0 · macOS Apple Silicon

2026-11-22 构建。最低系统版本为 macOS 13.0。解压 `PdmView-1.0.0-macos-arm64.zip` 后打开 `PdmView.app`。

```sh
make package-macos VERSION=1.0.0
```

- 产物为 `.out/release/1.0.0/macos-arm64/PdmView-1.0.0-macos-arm64.zip`，SHA-256 为 `3aee63187b528167ece5d9614cd085c720472696939e7443ea2182339985b5ec`。
- 猫头鹰图标原图为 [`assets/PdmView.png`](assets/PdmView.png)。应用通过 `CFBundleIconFile` 使用同名 ICNS，含 16 至 1024 像素的 10 个标准图标资源。图标文件纳入应用包和 ZIP 文件清单。
- macOS 15.8.1 arm64 上通过 ZIP 完整性及精确清单校验。解压后验证两处版本号、Mach-O 架构、图标文件字节和 ad hoc 签名完整性。通过 Launch Services 启动，观察到原生主窗口，运行中的应用图标与系统读取的应用包图标一致，正常退出。
- 本机未配置 Developer ID Application 证书。本次为 ad hoc 签名的本地发布包，未做 Apple 公证。Windows 桌面验收仍由 Windows 本机执行。

## 0.1.0 发布候选

2026-11-21 在 macOS Apple Silicon 上构建本地候选。平台验收范围见 [Phase 10 验收](docs/PHASE_10_ACCEPTANCE.md)。完整 Gate 5 待通过。

## GitHub Actions 构建与发行

将 [Package PdmView](.github/workflows/package.yml) 同步到 GitHub 默认分支后，在 Actions 中选择要构建的提交所在分支或标签，手动运行工作流并输入 `x.y.z` 版本号。工作流在私有仓库中也能运行。

| 构建目标 | 发行文件 |
| --- | --- |
| macOS Apple Silicon | `PdmView-<版本>-macos-arm64.zip`，含 `PdmView.app` 和 ad hoc 签名 |
| Windows x64 | `PdmView-<版本>-windows-amd64.zip`，含 `PdmView.exe` |
| Linux x64 | `PdmView-<版本>-linux-amd64.deb`，目标为 Ubuntu 24.04 LTS 桌面版，可用 `sudo apt install ./文件名.deb` 安装 |
| 对应源码 | `PdmView-<版本>-source.tar.gz`，含构建提交的源码、脚本和 Go 依赖模块源码 |

每个文件附 SHA-256 校验文件。下载 Actions 产物进行验收，并记录运行对应的提交 SHA。验收后，用该提交创建发行标签，将包、校验文件和对应源码一起附到 GitHub Release。macOS 包尚未完成 Apple 公证，Windows 和 Ubuntu 的桌面验收须单独记录。

## 构建与校验

从仓库根目录运行。macOS 和 Windows 打包命令均在 macOS 上执行。

```sh
make package-macos VERSION=0.1.0
make package-windows VERSION=0.1.0
```

在 Ubuntu 24.04 LTS x64 上可运行 `make package-linux VERSION=x.y.z` 构建 `.deb`，其中 `x.y.z` 替换为发行版本号。

产物在 `.out/release/<版本>/` 下。脚本输出本次生成的完整路径。已有同版本产物时会为新构建分配独立目录。

| 目标 | 归档与启动入口 | 签名与验收范围 |
|---|---|---|
| macOS Apple Silicon | `PdmView-0.1.0-macos-arm64.zip`，解压后打开 `PdmView.app` | ad hoc 签名，签名完整性校验通过。未做 Developer ID 签名、公证或下载隔离场景验收。已在 macOS 15.8.1 实测核心流程。 |
| Windows x64 | `PdmView-0.1.0-windows-amd64.zip`，解压后打开 `PdmView.exe` | 无 Authenticode 签名。PE32+ AMD64、Windows GUI 子系统及 ZIP 内容已核对，桌面验收延期。 |

macOS 上可用 `shasum -a 256` 核对归档，并在解压后运行 `codesign --verify --deep --strict PdmView.app`。Windows 上可用 `Get-FileHash -Algorithm SHA256` 核对候选归档和解压后的 EXE。

本次本地构建的校验和如下。路径相对于仓库根目录。

| 文件 | SHA-256 |
|---|---|
| `.out/release/0.1.0/macos-arm64.MRnni4/PdmView-0.1.0-macos-arm64.zip` | `4f36cd4381dc3134017ec6ce275a63f5665f791184f836a710216fbff9012d7d` |
| `.out/release/0.1.0/windows-amd64.Uf3rNl/PdmView-0.1.0-windows-amd64.zip` | `b7191c47424473d40ad3123b3167499441642a87c4afe58853434cb29bfbf08d` |
| `.out/release/0.1.0/windows-amd64.Uf3rNl/PdmView.exe` | `a607777e63487e364b6dda471c5ab28103892d0044cb379a97a2c294a29f6b6f` |

## 依赖与许可证

PdmView 自有代码采用 GNU GPL v3.0（仅第 3 版，`GPL-3.0-only`），正文见 [LICENSE](LICENSE)。macOS 归档在 `PdmView.app/Contents/Resources/LICENSE` 提供许可文本，Windows 归档将 `LICENSE` 放在 `PdmView.exe` 旁边，Linux 包安装到 `/usr/share/doc/pdmview/LICENSE`。旧版归档内容以上方构建记录为准。

正式候选使用 Go 1.27.4 和 MyGo v0.10.0。Lobe MyGo 只用于 Phase 0 的隔离验证。

| 组件 | 版本 | 随包许可证 |
|---|---|---|
| Go 标准库 | 1.27.4 | BSD-3-Clause 和 PATENTS |
| MyGo | v0.10.0 | MIT，另附 shader 包 Apache-2.0 |
| purego | v0.11.1 | Apache-2.0 |
| go-text/typesetting | v0.3.5 | Unlicense 或 BSD-3-Clause，另附 HarfBuzz MIT |
| x/image、x/text | v0.46.0、v0.43.0 | BSD-3-Clause 和 PATENTS |
| x/net | v0.59.0 | Windows 候选实际链接，附 BSD-3-Clause 和 PATENTS |

macOS 第三方许可证位于应用包的 `Contents/Resources/ThirdPartyNotices/`，Windows 位于 ZIP 根目录的 `ThirdPartyNotices/`，Linux 安装后位于 `/usr/share/doc/pdmview/ThirdPartyNotices/`。其中的 `modules.txt` 列出版本和原文校验和。二进制发行包排除 PDM、测试夹具和本地日志。

### 公开发行的对应源码

从确定的 Git 提交构建，并在二进制下载旁提供可免费获取的同版本对应源码。源码归档应包含该提交的项目源码、必要的构建与打包脚本，以及构建该发行包所需的第三方依赖源码与许可证。按 `go.mod`、`go.sum` 和各平台发行包内的 `modules.txt` 核对依赖版本，并用发行标签记录源码提交。历史构建的文件与校验和以上方版本记录为准。

Actions 生成的源码归档附带 `go-module-proxy/`。解压后，在源码根目录可以将 `GOPROXY` 指向 `file://` 加该目录的绝对路径，使用随包模块构建。离线时设置 `GOSUMDB=off`，Go 仍会核对 `go.sum` 中已记录的校验和。系统图形库和构建工具按对应平台的要求准备。

## Windows 本机验收交接

如需在 Windows x64 本机从同一源码构建，可在仓库根目录执行以下 PowerShell 命令。新构建应记录自己的 SHA-256。

```powershell
$env:CGO_ENABLED = '0'
go build -mod=readonly -trimpath -buildvcs=false -tags 'mygo_noinspector,mygo_nowebview' -ldflags '-s -w -H=windowsgui' -o .out\windows-local\PdmView.exe ./cmd/pdmview
Get-FileHash -Algorithm SHA256 .out\windows-local\PdmView.exe
```

1. 核对 Windows 版本、x64 架构、显示缩放、GPU 及候选的 SHA-256。解压、启动及退出，确认 D3D11 渲染。
2. 在没有偏好文件的测试配置中确认默认语言。用原生菜单双向切换，检查勾选、及时刷新、重启后偏好恢复，以及高 DPI 下两种语言的最小窗口。
3. 使用 Windows 本机的合成 PDM 测试原生打开和取消。核对中文与拉丁文字、画板定位、滚轮、搜索、详情和大图性能。
4. 对照两种语言的完整表信息和 SQL 剪贴板字节。验证原生 `.sql` 保存、取消、覆盖确认、扩展名、内容及文件权限提示。
5. 记录结果和所用二进制的 SHA-256，再更新 [Phase 10 验收](docs/PHASE_10_ACCEPTANCE.md#p10-02-windows-x64) 与 [任务状态](tasks/TASK_STATUS.md)。样本留在本机。

## 适用范围和已知限制

- PDM 输入为 UTF-8 XML。已用本地 PowerDesigner 16.5.0.3982 真实样本和公开 XML 导出验证结构。其他格式、外部模型引用及应用内布局对照见 [PDM 格式验证](docs/PDM_FORMAT.md)。
- SQL 实测目标为 MySQL 8.0.46。类型映射白名单、阻断规则和限制见 [类型映射](docs/MYSQL_TYPE_MAPPING.md)，执行证据见 [Phase 9 验收](docs/PHASE_9_ACCEPTANCE.md)。
- macOS 候选的最低系统版本为 13.0。PdmView 使用原生 UI，应用流程未创建 WebView。锁定的 MyGo v0.10.0 在 macOS 启动时直接加载系统 WebKit。隔离实验取消 MyGo 直接加载后，macOS 15.8.1 的 AppKit 仍经 `WritingToolsUI` 和 `GenerativeAssistantSettings` 间接加载 WebKit。Windows 候选使用 `mygo_nowebview` 移除 WebView2 后端。
- macOS 的 500 表密集画布已通过 180 秒实测。Ctrl 加滚轮的双向缩放、指针锚点及普通滚轮平移已通过 [原生滚轮验收](docs/PHASE_10_ACCEPTANCE.md#鼠标滚轮缩放)。P4-03 PowerDesigner 应用内对照仍为 `BLOCKED`。
