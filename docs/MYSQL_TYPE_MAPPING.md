# PDM 到 MySQL 8.0 的类型映射

## 输入与保真规则

类型生成器读取 PDM 领域模型的 `DataType`、`Length`、`Precision` 和 `Scale`。类型参数同时出现在 `DataType` 和独立属性中时，核对两处是否一致。PowerDesigner 的定点数 `Length` 表示总位数，`Precision` 表示小数点后的位数。例如 `DataType=decimal(20,2)`、`Length=20`、`Precision=2` 对应 `DECIMAL(20,2)`。[SAP PowerDesigner Data Modeling，Columns (PDM)](https://help.sap.com/doc/c7bcb6fe6e1b10148e8eca8d2c439245/16.6.8/en-US/data_modeling.pdf)

整数括号中的 `M` 是旧版显示宽度。`INT(10)` 和 `INT(20)` 均生成 `INT`，保留原整数范围。`DECIMAL(M,D)` 核对 `1≤M≤65`、`0≤D≤30`、`D≤M`。`TIME`、`DATETIME`、`TIMESTAMP` 的小数秒精度为 0–6。[MySQL 8.0 数值类型语法](https://dev.mysql.com/doc/refman/8.0/en/numeric-type-syntax.html) · [日期时间类型语法](https://dev.mysql.com/doc/refman/8.0/en/date-and-time-type-syntax.html)

`CHAR`、`VARCHAR` 的长度单位是字符，`BINARY`、`VARBINARY` 的长度单位是字节。目标字符集、InnoDB 行大小和索引字节上限仍会影响建表结果。[MySQL 8.0 字符串类型语法](https://dev.mysql.com/doc/refman/8.0/en/string-type-syntax.html) · [InnoDB 限制](https://dev.mysql.com/doc/refman/8.0/en/innodb-limits.html)

## 本地样本盘点

本地样本包含 13 张表、149 个字段、18 种原始类型写法。以下为按类型族汇总的字段数。

| 源类型 | 字段数 | MySQL 表达 |
|---|---:|---|
| `char(1/5/18)` | 30 | `CHAR(n)` |
| `varchar(20/35/50/60/80/100/130/150/200)` | 68 | `VARCHAR(n)` |
| `int(10/20)` | 17 | `INT` |
| `decimal(20,2)` | 6 | `DECIMAL(20,2)` |
| `date` | 4 | `DATE` |
| `datetime` | 15 | `DATETIME` |
| `text` | 9 | `TEXT` |

6 个定点字段均带 `Length=20`、`Precision=2`，均无独立 `Scale`。样本的字段定义未包含无符号、字符集或排序规则属性。映射测试另用合成输入覆盖这些修饰符及未知类型。

## 实现范围

| 类型族 | 支持及规范化 |
|---|---|
| 整数 | `TINYINT`、`SMALLINT`、`MEDIUMINT`、`INT`、`BIGINT`。兼容 `INT1/2/3/4/8`、`INTEGER`、`MIDDLEINT`、`BOOL`、`BOOLEAN` 别名 |
| 定点及近似数 | `DECIMAL`、`DEC`、`NUMERIC`、`FIXED`，以及 `FLOAT`、`DOUBLE`、`DOUBLE PRECISION`、`REAL`。`FLOAT(p)` 在 0–24 位时生成 `FLOAT`，在 25–53 位时生成 `DOUBLE` |
| 定长和变长字符串 | `CHAR`、`VARCHAR`、`BINARY`、`VARBINARY`，以及 `CHARACTER`、`CHAR VARYING`、`CHARACTER VARYING` |
| 大字段 | `TINYTEXT`、`TEXT`、`MEDIUMTEXT`、`LONGTEXT`，对应的四种 `BLOB`，以及 `JSON` |
| 时间和位值 | `DATE`、`TIME`、`DATETIME`、`TIMESTAMP`、`YEAR`、`BIT` |

从类型后缀或已识别的独立 XML 属性保留 `UNSIGNED`、`CHARACTER SET` 和 `COLLATE`。属性的标记语法和多来源冲突由生成器校验，目标库中是否存在该字符集或排序规则及其组合仍需核验。`CHARACTER SET binary` 会改变列的物理类型，因此要求源模型直接声明二进制类型。[MySQL 8.0 字符串类型语法](https://dev.mysql.com/doc/refman/8.0/en/string-type-syntax.html)

未知类型、非法参数、不支持的修饰符和冲突属性分别返回阻断错误。当前显式拦截 `ENUM`、`SET`、空间类型、`SERIAL`、`ZEROFILL` 和旧式 `FLOAT(M,D)`、`DOUBLE(M,D)`。其他数据库的同义名称只有在可确认语义一致时才纳入白名单。
