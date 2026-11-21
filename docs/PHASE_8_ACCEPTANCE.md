# Phase 8 MySQL 8.0 生成器验收

## P8-01 类型映射和未知类型诊断

- 本地 PDM XML 盘点与冻结领域模型的 Go 映射结果独立核对。149/149 个字段映射一致，18 种源写法归一为 17 种 SQL 表达，零阻断错误。业务样本只在本地读取，详见 [类型映射](MYSQL_TYPE_MAPPING.md)。
- `go test -count=1 ./internal/sqlgen`、`go vet ./internal/sqlgen` 和 `make format-check` 通过。合成场景覆盖未知类型、非法语法、参数越界、部分属性和冲突属性、别名、无符号、字符集、排序规则及禁止原样透传。

## P8-02 单表 DDL

- `GenerateTable` 从冻结的领域模型生成确定性 `CREATE TABLE`，保留字段和主键列顺序、NULL 语义、自增及注释。标识符优先采用 Code，Code 缺失时用 Name 并报告警告。阻断错误附带可定位的诊断并返回空 SQL。
- 默认值区分未设置、SQL `NULL`、字符串（含空字符串）、数值和受支持的表达式。未识别表达式、类型冲突及能够确定的越界值均阻断。表达式默认值使用 MySQL 8.0.13+ 语法。[MySQL 8.0 默认值规则](https://dev.mysql.com/doc/refman/8.0/en/data-type-defaults.html)
- 本地样本 13 张表的 DDL 重复生成一致。149 个字段声明的类型、NULL 属性及顺序、13 个主键顺序和 68 条注释逐项核对通过。样本中的默认值和自增场景由合成用例覆盖。
- `go test -count=1 ./internal/sqlgen ./internal/pdm`、对应 `go vet` 和 `make format-check` 通过。用例覆盖反引号、引号、反斜杠、Unicode、复合主键、未知类型阻断、默认值分类和自增约束。

## P8-03 索引和约束

- 索引生成保留复合列顺序、升降序、受支持的前缀、唯一性和索引注释。自增列资格通过实际输出的索引核对。无法表达的索引类型和已识别的不支持属性有阻断诊断。[MySQL 8.0 索引规则](https://dev.mysql.com/doc/refman/8.0/en/create-index.html)
- 外键编译保留约束名及有序的子列、父列映射，验证父键、类型和动作。目标使用完整的主键或唯一键。数字动作码和类型兼容性无法确认时阻断。[MySQL 8.0 外键限制](https://dev.mysql.com/doc/refman/8.0/en/create-table-foreign-keys.html)
- 本地业务样本的 13 个主键均只声明一次。两份本地合成 XML 覆盖 2 个复合主键、1 个降序前缀索引和 1 个带升降序的复合唯一索引。
- `go test -count=1 ./internal/sqlgen ./internal/pdm`、对应 `go vet` 和 `make format-check` 通过。补充用例覆盖同名主键镜像去重、不同名称的索引保留、缺列及重复列、JSON 索引、前缀过长、复合索引列数上限、类型不兼容、无效的 `SET NULL` 和自引用同列。
- 生成阶段检查可从模型确定的索引长度上限。继承字符集及目标实例页大小导致的字节限制随 P9-04 实测。

## P8-04 全库依赖和外键策略

- 依赖计划先收集 Model 与所有 Package 中的表，再解析外键。表与外键分别按最终 SQL 名称稳定排序，复用单表编译和渲染。确定性测试覆盖同名冲突、缺失表或列、缺失父键、类型不匹配、跨 Package、循环以及合法自引用。
- 外部表快捷方式、SQL 对象的解析结构错误、多根 Model 和无法映射的库或 Schema 限定生成带位置的阻断诊断。SQL 相关的未知 XML 项保留为警告。Package 仅用于组织表和关系。
- 本地 `yian.pdm` 的计划含 13 张表和 0 个外键，零阻断。合成 XML 的 2 张表、2 个跨 Package 循环外键和 4 个有序列映射逐项一致。通过忽略目录中的 `go test -overlay` 夹具核对，全库和单表的 `CREATE TABLE` 复用由包级测试断言。
- `go test -count=1 ./internal/sqlgen ./internal/pdm`、对应 `go vet`、`make format-check` 及 `git diff --check` 通过。

## P8-05 全库初始化 SQL

- `GenerateDatabase` 依次生成 `CREATE DATABASE IF NOT EXISTS`、`USE`、所有 `CREATE TABLE` 与延后的 `ALTER TABLE ... ADD CONSTRAINT`。库名优先用显式选项，其次取 Model Code，回退到 Model Name 时给出警告。限定库名和系统库名阻断。
- 冻结的 PDM 模型没有已验证的数据库级字符集与排序规则来源。生成器接受显式 `CharacterSet` 和 `Collation` 选项，检查 ASCII 语法、字符集识别和排序规则前缀匹配。缺省选项沿用目标实例的默认设置。指定排序规则的完整名称可用性由 P9-04 在目标 MySQL 8.0 实例复核。[MySQL 8.0 CREATE DATABASE 规则](https://dev.mysql.com/doc/refman/8.0/en/create-database.html)
- 本地 `yian.pdm` 生成 13 条 `CREATE TABLE`，与 13 次单表生成的对应语句逐字节一致。合成循环 XML 的 2 条 `CREATE TABLE` 同样逐字节一致，2 条 `ALTER TABLE` 都在全部建表语句之后，约束名、动作及 4 个列映射均与 XML 一致。重复生成两份脚本均逐字节一致。
- 两份公开导出在本地复测，分别报告 26 个不支持的外键动作和 5 个不支持的默认表达式。两次均返回空 SQL。合成用例覆盖数据库名转义、非法字符集参数、排序规则与字符集错配、改换输入顺序后的确定性和阻断时无部分输出。

## Phase 8 集成验收

- 2026-11-21 使用 Go 1.27.4、macOS arm64 和 `CGO_ENABLED=0`。`go test -count=1 ./internal/sqlgen ./internal/pdm`、`make check`（格式、全项目测试与 vet）和 `git diff --check` 通过。
- 本地业务样本仅由本机验证程序读取。`shasum -a 256 -c .local/phase3/sample.sha256` 通过。验收程序和业务样本均在 Git 忽略范围内。
- Phase 8 收尾时 MySQL 8.0 实际执行为 **NOT RUN**。后续 Gate 4 的执行和元数据核对见 [P9-04 验收记录](PHASE_9_ACCEPTANCE.md#p9-04-mysql-80-实际执行)。P4-03 的 PowerDesigner 应用内对照继续为 **BLOCKED**。
