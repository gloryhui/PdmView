# Phase 9 SQL 预览、导出与 MySQL 验收

## P9-01 原生双语 SQL 预览

- 「数据库 SQL」打开独立工作区页面。后台使用当前 PDM 文档生成，返回或更换模型时丢弃过期结果。预览使用 MyGo v0.10.0 原生只读代码编辑器，保留 SQL 高亮和横纵滚动。
- 在 macOS 800×600 DIP 窗口上双向切换中英文。合成夹具的 2 张表、31 个字段和 2 个外键统计一致，编辑器内 1,445 个 UTF-8 字节保持一致。输入不能修改 SQL，滚动到底仍显示两条外键语句。返回后恢复工作区。
- 错误夹具将一个字段改为未知类型。两种语言均显示 1 个错误、0 个警告，定位到 XML 第 36 行，预览不显示部分 SQL。
- `go test -count=1 ./internal/ui ./internal/i18n ./cmd/pdmview`、对应 `go vet`、格式检查和原生 `e2e` 构建通过。原生快照及截图保存在忽略目录 `.out/phase9/`。文案审核保留按钮、统计、进度和可定位诊断，编辑器标题仅供辅助技术读取。

## P9-02 SQL 复制与文件导出

- 在 macOS 原生窗口中，两种语言的全库剪贴板均与预览逐字节一致（1,445 字节）。详情区的单表建表 SQL 两次复制均为同一条 944 字节语句，且与脚本中的对应语句一致。验收结束后恢复原系统剪贴板。
- 原生保存面板取消后未创建文件。初测发现 macOS 自动追加扩展名，修正默认名后，双语面板均以建议名直接保存 `phase8_fixture.sql`，两份文件各 1,445 字节且与预览逐字节一致。系统按钮在本机均为 `Cancel` / `Save`。应用内成功反馈随语言更新。
- 定向测试、对应 `go vet`、格式检查与原生 `e2e` 构建通过。用例涵盖警告可见、阻断禁用、取消、过期结果、扩展名、拒绝非普通文件、写入失败保留原文件及首次复制的回归。Windows 实际保存和替换文件随 P10 验收。

## P9-03 Golden Tests

- 四份静态 SQL 文件固定一份完整脚本和三份单表语句。完整脚本 1,317 字节，覆盖复合主键、候选唯一键、降序前缀、跨 Package 循环与自引用。每份单表语句须在全库中逐字节出现一次，重复生成或调换 Package 与外键集合顺序均保持一致。
- 夹具覆盖空字符串、数值、SQL `NULL`、`CURRENT_TIMESTAMP(6)`、`JSON_ARRAY()`、多语言注释、反引号、反斜杠和换行转义。测试只读静态期望值，业务样本未纳入夹具。
- `go test -count=1 -run Golden -v ./internal/sqlgen`、`go vet ./internal/sqlgen`、格式检查通过。

## P9-04 MySQL 8.0 实际执行

- 2026-11-21 使用本机 MySQL 8.0.46（macOS arm64）。测试各自初始化独立数据目录，查询确认所连接实例使用本次创建的 Unix socket 与数据目录，并禁用 TCP、MySQL X Protocol 和二进制日志。测试结束后两个实例均已关闭，临时目录由测试框架清理。
- Golden 脚本执行成功。`information_schema` 核对了 1 个库的字符集和排序规则、3 张 InnoDB 表、14 个字段、10 条索引列记录及 9 条键列记录。降序索引的前缀长度和注释、3 个外键的引用目标和动作均匹配。插入测试核对自增、空字符串、换行与反斜杠、时间、JSON、数值和 `NULL` 默认值。缺失父行的插入返回 MySQL 错误 1452。
- 本地业务样本生成 SQL 并执行成功。独立 XML 读取程序逐项核对 13 张表、149 个字段的名称、顺序、类型、NULL 和默认值、13 个主键及其主索引记录、68 条表或字段注释。源样本中没有显式二级索引或外键。另执行 P9-02 原生导出的合成脚本，2 个循环外键的 4 个有序列映射和 `RESTRICT` / `CASCADE` 动作核对通过。
- 可复跑合成测试。`PDMVIEW_MYSQL80_BIN` 指向本机 MySQL 8.0 的 `bin` 目录。`TMPDIR` 指向项目已忽略且路径足够短的目录。业务样本独立对照脚本和 overlay 位于本机 `.local/phase9/`。

```sh
TMPDIR="$PWD/.local" PDMVIEW_MYSQL80_BIN=/opt/homebrew/opt/mysql@8.0/bin CGO_ENABLED=0 \
  go test -count=1 -run '^TestMySQL80$' -v ./internal/sqlgen
```

## Phase 9 集成验收

- Gate 4 在上述 MySQL 8.0.46 实例及测试配置下通过。`make check`、`CGO_ENABLED=0 go test -tags e2e ./cmd/pdmview`、`git diff --check` 和本地样本 SHA-256 复查通过。
- 业务样本仅在本机读取和验证。P4-03 的 PowerDesigner 应用内对照保留 `BLOCKED`。Windows 原生保存验收归 P10。
