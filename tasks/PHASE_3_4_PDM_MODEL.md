# Phase 3～4：PDM 格式研究、解析与统一模型

## P3-01 真实样本盘点
- 记录每个样本的 PowerDesigner 版本（若已知）、文件大小、编码、Diagram 数量。
- 生成 SHA-256 用于本地样本身份核对，不记录文件内容。
- 真实样本默认放在 `.local/fixtures/`，加入 `.gitignore`。
- 建立脱敏和模拟样本策略。

## P3-02 XML 结构与版本研究
- 使用 XML 工具查看根节点、命名空间、对象集合和引用。
- 找出 Model、Package、Table、Column、Key、Index、Reference、Diagram、Symbol 的真实结构。
- 识别对象 ID、引用 ID、坐标单位、路径点结构。
- 记录版本差异。
- 每个关键结论附样本路径、XML 节点 XPath/结构说明和验证证据。
- 不得仅凭网上示例推断全部版本结构。

输出：`docs/PDM_FORMAT.md`。

## P3-03 原始 XML 表示
建立能够保留对象 ID、原始类型、关键属性、未知节点诊断的内部表示。
避免在解析时丢失 Name/Code、空值/缺失值、默认值原文。

## P3-04 Model/Package/Table
解析模型元数据、Package 嵌套、表对象、对象归属。
验收：表和 Package 数量与 PowerDesigner 对照。

## P3-05 Column/Key/Index/Reference
解析字段顺序、类型、长度、精度、小数位、NULL、默认值、注释、自增、主键、索引、外键字段映射。
验收：抽样对照准确；对象引用缺失有诊断。

## P3-06 Diagram/Symbol/路径
解析多个 Diagram、Symbol 与实际表对象的关联、X/Y/宽高、关系线和控制点。
区分模型对象与画板图形对象；同一表可能在多个 Diagram 中有不同 Symbol。
验收：可以在测试报告中逐个核对 Diagram 和 Symbol 映射。

## P3-07 诊断与测试
测试：
- 正常文件。
- 截断 XML。
- 未知节点。
- 缺失引用。
- 重复 ID。
- 空注释。
- 中文与特殊字符。
- 多 Package、多 Diagram。
- 超大文件。
输出解析统计与诊断列表。

## P4-01 Domain Model 冻结
主 Agent 根据真实样本确定最终模型和接口。冻结后任何 breaking change 必须先记录决策。

## P4-02 Diagram 到 Graph 映射
将 Symbol 映射为 Graph Node，将关系图形映射为 Graph Edge；保留原始几何路径。
不得将一个表的多个 Diagram Symbol 合并成一个全局坐标。

## P4-03 原始布局核对
与 PowerDesigner 对照表位置、尺寸、字段展示顺序、关系线。
缺失布局时才 fallback 自动布局，并在 UI 中显示“自动布局”状态。
