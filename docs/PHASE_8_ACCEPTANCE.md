# Phase 8 MySQL 8.0 生成器验收

## P8-01 类型映射和未知类型诊断

- 本地 PDM XML 盘点与冻结领域模型的 Go 映射结果独立核对。149/149 个字段映射一致，18 种源写法归一为 17 种 SQL 表达，零阻断错误。业务样本只在本地读取，详见 [类型映射](MYSQL_TYPE_MAPPING.md)。
- `go test -count=1 ./internal/sqlgen`、`go vet ./internal/sqlgen` 和 `make format-check` 通过。合成场景覆盖未知类型、非法语法、参数越界、部分属性和冲突属性、别名、无符号、字符集、排序规则及禁止原样透传。

## P8-02 单表 DDL

- `GenerateTable` 从冻结的领域模型生成确定性 `CREATE TABLE`，保留字段和主键列顺序、NULL 语义、自增及注释。标识符优先采用 Code，Code 缺失时用 Name 并报告警告。阻断错误附带可定位的诊断并返回空 SQL。
- 默认值区分未设置、SQL `NULL`、字符串（含空字符串）、数值和受支持的表达式。未识别表达式、类型冲突及能够确定的越界值均阻断。表达式默认值使用 MySQL 8.0.13+ 语法。[MySQL 8.0 默认值规则](https://dev.mysql.com/doc/refman/8.0/en/data-type-defaults.html)
- 本地样本 13 张表的 DDL 重复生成一致。149 个字段声明的类型、NULL 属性及顺序、13 个主键顺序和 68 条注释逐项核对通过。样本中的默认值和自增场景由合成用例覆盖。
- `go test -count=1 ./internal/sqlgen ./internal/pdm`、对应 `go vet` 和 `make format-check` 通过。用例覆盖反引号、引号、反斜杠、Unicode、复合主键、未知类型阻断、默认值分类和自增约束。
