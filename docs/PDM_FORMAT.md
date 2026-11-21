# Phase 3 PDM XML 格式验证

## 本地样本盘点

2026-11-21 在本机只读验证 `pdm_file/yian.pdm`。文件为 132,997 字节，无 BOM，XML 1.0，声明编码为 UTF-8。`PowerDesigner` 处理指令的 `version` 为 `16.5.0.3982`，签名为 `PDM_DATA_MODEL_XML`。样本含 1 个物理画板，DBMS 快捷方式声明为 MySQL 5.0。XML 编码以 XML 声明为准。

`scripts/inspect-pdm.py` 使用 Python 标准库 Expat，输出白名单版本字段、结构和计数。Python 3.9.6 盘点通过，`xmllint --nonet --noout` 独立检查也通过。样本及其 SHA-256 核验文件由 Git 忽略。核验值只存于本地 `.local/phase3/sample.sha256`。提交中的测试数据采用独立虚构模型，覆盖样本缺少的结构。日后如需共享真实导出，需先脱敏并重新核对结构和计数。

```sh
python3 scripts/inspect-pdm.py pdm_file/yian.pdm
xmllint --nonet --noout pdm_file/yian.pdm
shasum -a 256 -c .local/phase3/sample.sha256
```

## 本地 XML 结构与引用

表中 `M` 表示 `/Model/o:RootObject/c:Children/o:Model`。`o`、`c`、`a` 分别绑定 `object`、`collection`、`attribute`。解析按命名空间与局部名称查询，允许同一绑定换用其他前缀。首层 `Model` 无命名空间。

| 内容 | 样本 XPath / 结构 | 本地验证 |
|---|---|---|
| Writer 版本 | `/processing-instruction('PowerDesigner')`，取 `version` 与 `signature` | 1 个版本声明，实际仅验证 `16.5.0.3982` |
| 模型、DBMS | `M` 的 `a:Name`、`a:Code`、`a:ObjectID`，DBMS 在 `M/c:DBMS/o:Shortcut` | 1 个模型和 1 个 DBMS 快捷方式，快捷方式的 `a:Name` 与 `a:Code` 独立 |
| 表与字段 | `M/c:Tables/o:Table/c:Columns/o:Column` | 13 表、149 字段，顺序取 XML 集合顺序。68 个表或字段注释，其中 55 个是字段注释 |
| 类型与空值 | `o:Column/a:DataType`、`a:Length`、`a:Precision`、`a:Column.Mandatory` | 13 个 Mandatory 值为 `1`。6 个带两个数值参数的类型均对应 `DataType(p,s)`、`Length=p`、`Precision=s`。字段保留 PowerDesigner 原属性 |
| 键 | `o:Table/c:Keys/o:Key/c:Key.Columns/o:Column[@Ref]`，主键由 `o:Table/c:PrimaryKey/o:Key[@Ref]` 指向 | 13 个 Key、13 个主键声明。全部键列归属各自的表 |
| 画板 | `M/c:PhysicalDiagrams/o:PhysicalDiagram`，默认画板通过 `M/c:DefaultDiagram/o:PhysicalDiagram[@Ref]` | 1 画板 |
| 表图元 | `o:PhysicalDiagram/c:Symbols/o:TableSymbol`，表引用在 `c:Object/o:Table[@Ref]` | 8 个表图元。另有 1 个 `o:RectangleSymbol` 注释图元 |
| 表图元坐标 | `o:TableSymbol/a:Rect` 为 `((x1,y1), (x2,y2))` | 8 个矩形均有四个整数且两角有序。宽高为两角的差值，保留原始整数坐标和文本。物理单位、屏幕换算留待后续实机对照 |
| 定义与引用 | `o:*[@Id]` 是文档内定义，`o:*[@Ref]` 指向 `Id`。`a:ObjectID` 是独立属性 | 2,039 个 XML 元素，190 个定义，36 处引用。重复 ID、缺失引用和类型不匹配均为 0 |

## 未覆盖结构与版本范围

- 用户样本没有 Package、Index、Reference、ReferenceSymbol，也没有显式默认值、自增或空注释。嵌套 Package、多 Diagram、空值与缺失值、索引列、外键字段配对、线段和控制点用合成夹具回归。
- 本机额外只读检查三份公开 XML 导出的结构。12.5.0.2169 与 15.1.0.2850 的样例使用 `a:Mandatory`，16.5.0.3982 的样例使用 `a:Column.Mandatory`。公开样例同时出现 `a:Identity` 与 `a:DefaultValue`。这些是样本间观察到的变化，兼容范围以实际测试为准。[12.5 公开导出](https://github.com/ponxu/blog4j/blob/d9be3eaa3e4f972ec11732ddc3fb238fafb6ec85/doc/core.pdm) · [15.1 公开导出](https://github.com/sosoapi/sosoapi-web/blob/213d470f12e35b092f28db5fbc57281accb34305/doc/sosoapi.pdm) · [16.5 公开导出](https://github.com/yilezhu/Czar.Cms/blob/3b4a2c75d4ddb6104c42416d546299632216c5e5/pdm/Czar.Cms.pdm)
- 公开样例的 Package 通过 `c:Packages/o:Package` 归属父级，包内同样可有 `c:Tables`、`c:References` 和 `c:PhysicalDiagrams`。索引列通过 `c:Indexes/o:Index/c:IndexColumns/o:IndexColumn/c:Column/o:Column[@Ref]` 取顺序，唯一性使用 `a:Unique`。16.5 样例还出现索引的 `c:LinkedObject`，关联键或外键。
- 外键在 `c:References/o:Reference`，`c:ParentTable`、`c:ChildTable` 和 `c:ParentKey` 提供端点。`c:Joins/o:ReferenceJoin` 下 `c:Object1/o:Column[@Ref]` 是主端字段，`c:Object2/o:Column[@Ref]` 是从端字段。已对照列的实际归属。15.1 公开样例含指向 `o:Shortcut` 的父端，本阶段保留引用并报告外部对象解析限制。
- `o:ReferenceSymbol/c:Object/o:Reference[@Ref]` 指向逻辑外键，`c:SourceSymbol`、`c:DestinationSymbol` 指向本画板的 `o:TableSymbol`。可核对的源端对应从表，目标端对应主表。`a:ListOfPoints` 为 `((x1,y1),(x2,y2),...)`，公开样例观察到 2 至 4 个整数点。路径点按原序保存，点是否为折点或曲线控制点由原属性和后续实机研究决定。
- 本机尚未进行 PowerDesigner 应用内对照。样本统计与人工结构对照以 XML 工具为独立基准。跨版本、画板物理单位及 Gate 1 的应用内对照需要更多样本和 PowerDesigner 实测。

## 解析验收

- `pdm_file/yian.pdm` 经 Go 解析器得到 1 模型、0 Package、13 表、149 字段、13 主键、1 画板和 8 个表图元。逐表核对了 XML 顺序、归属、Name 和 Code，逐字段核对了类型、长度、Precision 原值、注释和默认值的存在性。13 条非空标记、55 条字段注释和 6 个 Precision 值与 XML 一致。逐个核对了图元的表引用、矩形两角及默认画板引用。
- 下表中的统计与独立 Expat 盘点一致。公开导出中的 37 个表矩形、23 条关系的原始路径点及各自引用逐项核对，通过另一套解析方法检查数值和顺序。

| XML 样本 | Package | 表 | 字段 | 索引 | 外键 | 画板 | 表图元 | 关系图元 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 用户本地 16.5.0.3982 | 0 | 13 | 149 | 0 | 0 | 1 | 8 | 0 |
| 公开导出 12.5.0.2169 | 0 | 4 | 17 | 0 | 2 | 1 | 4 | 2 |
| 公开导出 15.1.0.2850 | 4 | 22 | 213 | 3 | 16 | 5 | 25 | 16 |
| 公开导出 16.5.0.3982 | 0 | 8 | 100 | 10 | 5 | 3 | 8 | 5 |

解析器以 UTF-8 XML 为输入，接受 UTF-8 BOM。损坏、截断或格式不符的文件返回错误。输入上限为 256 MiB、128 层嵌套及 100 万个元素。对象类型、字段顺序和相应原始 XML 元素保留在解析结果中，未知节点、缺失引用和其他语义问题以诊断返回。`a:ControlPoints` 的独立表示由合成夹具验证，公开样例中的 `a:ListOfPoints` 按原序保存。

使用 `go run ./cmd/pdm-inspect pdm_file/yian.pdm` 可在本机复现解析统计。命令输出 JSON，包括版本、计数及诊断的代码、结构路径和行号。致命错误产生非零退出码，报告中保留安全的错误代码。普通诊断允许继续查看统计。输出省略业务名称、对象 ID、注释、默认值和文件路径。

用户样本的解析报告为 190 个原始定义、0 条诊断。公开 12.5 导出为 0 条诊断，公开 15.1 导出识别到 9 处外部引用或声明、8 处未知节点，公开 16.5 导出识别到 2 处未知节点。未知节点仍保留在原始树，外部模型保持未加载。真实样本的 SHA-256 在实测后复核一致。

`make check` 通过格式检查、全仓 Go 测试和 vet。回归覆盖截断、未知属性与命名空间、缺失引用、重复 ID、空注释、中文及 XML 转义、多 Package、多 Diagram 和 11,000 字段的确定性合成模型。命令另验证了非普通文件返回 `not-regular-file`，超过输入上限的文件返回 `document-too-large`。

在 Apple M4、macOS 15.8.1 与 Go 1.27.4 上，用默认优化构建运行本地合成压力样本。样本为 31,431,470 字节，解析出 1,000 表、50,000 字段、51,002 对象定义，诊断为 0。一次 `/usr/bin/time -lp` 实测的进程总耗时为 0.30 秒，峰值 RSS 为 193.5 MiB，含进程启动和 JSON 输出。日志保存在 `.out/phase3/large-final-time.log`，样本与日志留在本机。

## PDM 副本保存

原生表属性窗口以打开时的 XML 字节作为副本来源，按文档内对象 ID 匹配当前表及所属字段。现有对象的 `Name`、`Code` 和 `Comment` 可以保存。写入器按 XML 标记的字节范围替换属性文本，必要时插入缺失属性。未编辑的区域直接复制原字节。旧文件中的重复 ID、重复目标属性或含嵌套标记的目标文本会拒绝修改。另存先在目标目录写完临时文件，再用硬链接以独占名称发布完整副本。目标文件系统需支持硬链接。

自 0.1.2 起，首次另存成功后工作区采用副本的路径和最新字节，保留当前画板、视口与选择。后续保存复核磁盘上的副本内容，再用同目录的临时文件替换。应用配置目录以副本路径和内容的 SHA-256 识别已保存的副本，重新打开时恢复直接保存。原始文件、此前未登记的副本及在应用外发生内容变化的副本以只读方式打开。

- 本地 `yian.pdm` 的未编辑副本与原始 132,997 字节一致。表和字段各修改名称、编码和注释后，使用应用的加载入口重开副本，六项修改均读回，原有画板和 8 个表图元的位置、尺寸、样式和字段显示数量一致。原文件的 SHA-256 校验通过。
- 合成回归覆盖 BOM、CRLF、前缀、XML 转义、缺失注释、自闭合注释、键引用、未知扩展节点、非法编辑和禁止覆盖已有目标。原生界面回归验证保存请求只收集三类属性。`make check` 通过。
- 用合成模型在 macOS 实际打开原生保存面板。英文保存无编辑副本，中文编辑注释后再次另存，两次均实际写出 `.pdm`。对照了副本内容、画板 XML 和原始文件。PowerDesigner 往返验证及 Windows 桌面验证待进行。
- `make package-macos VERSION=0.1.1` 生成 Apple Silicon 测试包。ZIP 完整性、临时签名和包内原生主窗口启动通过。
- 0.1.2 本机回归用 `yian.pdm` 连续编辑两张表并重启应用。只出现一次原生另存面板，两张表的修改保存在同一个文件，重启后仍可直接保存。1 张画板、8 个原始图元和源文件校验一致。定向测试覆盖外部内容变化及符号链接保护，`make check` 通过。
- `make package-macos VERSION=0.1.2` 生成 Apple Silicon 测试包。再次验证了签名、ZIP 完整性及包内原生主窗口启动。
