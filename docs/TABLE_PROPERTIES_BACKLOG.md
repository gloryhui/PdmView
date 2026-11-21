# 表属性后续范围

本记录依据用户提供的 PowerDesigner 表属性截图。当前版本的原生表属性窗口提供常规、字段、索引、键和 SQL 预览，窗口内可以临时编辑和复制。以下六个页签留待后续实现。截图中的空列表提供了列名和界面结构，数据映射需使用包含实际条目的样本验证。

| 页签 | 截图可见字段与区域 | 后续实现与验收 |
| --- | --- | --- |
| Triggers（触发器） | Name、Code、G | 解析表关联的触发器和生成标记。原生列表打开只读详情，展示触发时机、事件和定义。用有触发器与空列表的样本核对。 |
| Procedures（存储过程） | Name、Code | 解析表关联的存储过程引用。支持从表属性查看已解析的过程名称、编码和定义，区分空列表与未解析引用。 |
| Physical Options（物理选项） | Syntax、Items、SQL。候选项为 `type`、`auto_increment`、`avg_row_length`、`checksum`、`min_rows`、`max_rows`、`pack_keys`、`password`、`delay_key_write`、`row_format`、`raid_type`、`raid_chunks`、`raid_chunksize`、`union`、`insert_method`、`data directory`、`index directory` | 读取表实际配置及其源 DBMS 版本。分别展示候选选项、已配置值和对应 SQL，验证旧版选项的适用性。对密码类值使用遮蔽显示，并与日志和剪贴板隔离。 |
| MySQL | Temporary table | 从 PDM 扩展属性读取临时表标记，在 MySQL 模型中只读展示。加入 SQL 生成前，单独验证 MySQL 8.0 的临时表与约束语义。 |
| Notes（备注） | Description、Annotation | 分别解析与展示描述和批注，保留段落和换行。用原生只读文本控件验证中文、长文及空值。 |
| Rules（规则） | Name、Code | 解析表与业务规则的关联，支持查看规则名称、编码和已解析定义。记录失效引用，并验证规则映射到约束的适用条件。 |

常规页另有 Stereotype、Owner、Number、Row growth rate、Dimensional type、Generate、Keywords。索引页另有 U、P、F、A 标志栏。后续应先核实源属性、标志含义及缺失值，再决定展示和 SQL 生成行为。物理选项截图来自旧 MySQL 模型，MySQL 8.0 的默认值和语法沿用项目生成规范。
