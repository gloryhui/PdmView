# Phase 0：项目勘察与技术决策

## P0-01 仓库与环境勘察
**目标：** 确认工作区、现有代码、Go 环境和可用构建工具。

步骤：
1. 检查当前仓库目录、Git 状态、已有文件。
2. 检查 `go version`、`go env`、CGO、编译器和平台 SDK。
3. 检查是否存在真实 PDM 样本；只记录路径和文件元信息，不打印敏感内容。
4. 检查是否有 MySQL 8.0/Docker 测试环境。
5. 输出 `docs/ENVIRONMENT.md`。

验收：
- 版本和环境记录准确。
- 未擅自覆盖现有文件。
- 真实 PDM 未复制到公开目录。

## P0-02 Go/MyGo API 调研
**目标：** 用真实代码验证 MyGo Native UI 的能力和当前 API。

步骤：
1. 查阅 MyGo 当前官方文档、GitHub 源码和示例。
2. 确认当前 Go 版本要求和 MyGo 兼容版本。
3. 确认 Native UI 启动方式。
4. 确认 Painter、Canvas、Transform、Clip、Path、Text API。
5. 确认鼠标滚轮、拖动、命中测试、事件分发 API。
6. 确认剪贴板、文件选择器、DPI/屏幕缩放 API。
7. 确认 macOS Metal、Windows D3D11 的实际构建要求。
8. 确认 Lobe MyGo 是 Native UI 组件库还是涉及 Web 技术；只复用适配的原生组件。
9. 编写最小程序并真实运行。

输出：`docs/MYGO_API_VERIFICATION.md`。

验收：
- 所有使用的 API 都来自当前版本官方文档或源码。
- 最小程序编译运行成功。
- 不使用 WebView。
- 记录无法验证的能力和风险。

## P0-03 项目骨架与规范
步骤：
1. 初始化 Go Module。
2. 建立目录结构。
3. 创建 README、架构文档、任务表。
4. 配置 gofmt、go vet、测试命令。
5. 记录依赖版本。
6. 建立基础日志和错误规范。

验收：
- `go test ./...` 可运行。
- `go vet ./...` 可运行。
- 空项目结构没有伪造业务实现。
