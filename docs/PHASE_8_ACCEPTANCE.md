# Phase 8 MySQL 8.0 生成器验收

## P8-01 类型映射和未知类型诊断

- 本地 PDM XML 盘点与冻结领域模型的 Go 映射结果独立核对。149/149 个字段映射一致，18 种源写法归一为 17 种 SQL 表达，零阻断错误。业务样本只在本地读取，详见 [类型映射](MYSQL_TYPE_MAPPING.md)。
- `go test -count=1 ./internal/sqlgen`、`go vet ./internal/sqlgen` 和 `make format-check` 通过。合成场景覆盖未知类型、非法语法、参数越界、部分属性和冲突属性、别名、无符号、字符集、排序规则及禁止原样透传。
