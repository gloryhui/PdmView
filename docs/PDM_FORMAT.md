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

- 当前文件没有 Package、Index、Reference、ReferenceSymbol，也没有显式默认值、自增或空注释。嵌套 Package、多 Diagram、空值与缺失值、索引列、外键字段配对、线段和控制点用合成夹具回归。
- 本阶段输入范围为可识别的 PowerDesigner XML。当前只有一份 16.5 PDM 实测样本，暂无跨版本差异结论。
- 解析契约中，外键 Join 以成对的主、从表字段记录，索引保留 XML 中的索引列顺序。画板端点以图元 ID 关联，逻辑外键以模型对象 ID 关联。对应 XML 路径由合成夹具覆盖，待真实样本对照。
- 本机尚未进行 PowerDesigner 应用内对照。样本统计与人工结构对照以 XML 工具为独立基准。跨版本、画板物理单位及 Gate 1 的应用内对照需要更多样本和 PowerDesigner 实测。
