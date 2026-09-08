# protodb Rows Scan 性能优化

## 当前实现

本实现基于 Go 1.27.1 和仓库使用的 pgx v5.11.0，保留 `go 1.25.0` 最低版本。
Phase 1 的通用 `sql.Scanner` 路径继续存在；Go 1.27 的优化通过 pgx 已实现的
`driver.RowsColumnScanner` 接入，不需要额外注册 `protodbdriver`。

本文件替代此前 Phase 2/3 的接口草案。标准库实际接口为：

```go
type RowsColumnScanner interface {
    Rows
    NextRow() error
    ScanColumn(scanCtx ScanContext, index int, dest any) error
}
```

实现该接口后，`database/sql` 调用 `NextRow` 而不是 `Rows.Next`。
`ScanColumn` 返回 `driver.ErrSkip` **不会自动回退**；驱动必须自行完成转换。
`sql.ConvertAssign` 可以用于驱动的兼容转换，但不会天然消除装箱或分配。

## 优化路径

| 场景 | 实际路径 |
|---|---|
| Go 1.25/1.26 或非 pgx 驱动 | 原有 Phase 1 扫描 |
| Go 1.27 + pgx，标量列 | pgx 的 RowsColumnScanner → ConvertAssign → 原有 protoFieldReceiver |
| Go 1.27 + pgx，匹配的内置数组列 | 请求二进制数组 → pgx ArraySetter → protobuf repeated 字段 |
| 生成代码的公开 repeated string 字段 | 直接填充独立 []string，再一次性赋值，减少逐元素反射 |
| 动态/opaque 消息及其他匹配的 repeated 标量 | pgx 元素接收器 → protoreflect.List |
| JSON/JSONB、自定义数组类型 | 原有 JSON/文本转换路径 |
| 内置数组与 protobuf 类型不匹配 | pgx 原生接收数组后转回文本，复用原有赋值规则 |
| 未知数组列 | 原生接收并忽略，避免 pgx 对二进制数组转普通字符串的限制 |

支持的内置数组为 bool、bytea、int2/int4/int8、float4/float8、text/varchar/bpchar。
只有 PostgreSQL 元素类型与 protobuf kind 匹配时才直接赋值；例如 int4[] → repeated int32。
numeric[]、枚举数组、自定义类型和 JSON 消息没有在本次增加专用解码器。

标量原生接收器的原型虽然减少分配，但 pgx v5.11.0 的目标类型反射判断增加了 CPU 开销，
所以没有默认启用该原型。数组必须配合二进制结果格式才能避免文本解析；
仅把 sql.Scanner 改成原生接收器并不保证更快。

生成的公开 string 列表以 protobuf 字段编号查找对应 Go 字段，而不是依赖列名大小写。
类型、字段索引、接收器和临时列表都属于当前扫描器，没有新增跨消息的全局字段缓存。
每次结果采用独立切片，字符串和字节数据不会保留数据库行缓冲区。
同一个扫描器应顺序扫描同一消息类型，不应并发使用。

## 接入

以下高层入口在能确认底层是 pgx 的情况下自动使用优化：

- 表查询和 service 自定义查询；
- SelectOne、Insert/Delete/Update 的返回行；
- Update/PartialUpdate 的 old/new 成对返回行。

普通 `*sql.DB` 和包裹它的 `*sqldb.DBWithDialect` 可确认驱动类型。
裸 `*sql.Tx` 以及仅包含事务与方言信息的包装器无法暴露实际驱动，继续使用通用路径；
不能仅凭 PostgreSQL 方言假设底层驱动是 pgx。

手动查询时，配对使用查询辅助函数和扫描器：

```go
rows, err := crud.QueryProtoRowsContext(ctx, db, query, args...)
if err != nil {
    return err
}
defer rows.Close()

scanner, err := crud.NewDbRowScannerForDB(db, rows, prototype, nil, nil)
if err != nil {
    return err
}
for rows.Next() {
    msg := prototype.ProtoReflect().New().Interface()
    if err := scanner.Scan(rows, msg); err != nil {
        return err
    }
    // 消费 msg。
}
return rows.Err()
```

`QueryProtoRows[Context]` 会在 Go 1.27 的已知 pgx 连接上请求内置数组二进制格式，
因此其结果须与 `NewDbRowScannerForDB` 或 `NewPGXDbRowScanner` 配对，
不能假设普通 `sql.Scanner` 能接收这些二进制数组。
用户显式传入的 pgx 查询格式选项优先于辅助函数的默认设置。

对于已确认来自 pgx 的事务或预编译查询，可显式使用 `NewPGXDbRowScanner`。
它兼容 pgx 的文本和二进制数组，但不会在查询完成后改变传输格式；
事务查询要获得二进制数组收益，需要在查询时自行传入相应 pgx 结果格式选项。
简单协议只能传输文本，不承诺与二进制路径相同的性能。

原有 `NewDbRowScanner`、`DbScan2ProtoMsg`、`DbScan2ProtoMsgx2` 的签名和通用用途保留。

## NULL、转换与兼容性

- NULL 数组和空数组清空 repeated 字段；NULL 数组元素映射为 protobuf 标量默认值。
- 字符串数组保留空白、引号、反斜杠以及字面量 `"NULL"`。这也修正了旧文本解析器把带引号
  的 NULL 字符串当作空值、裁剪字符串空白的问题。
- 匹配的整数数组在写入 int32/uint32/uint64/enum 前检查范围，不进行静默截断。
- 直接接收器拒绝多维数组，因为 protobuf repeated 字段不能表达维度。
- 不承诺数组、JSON 或消息字段零分配；驱动与 protobuf 仍有必要的解码和内存分配。

## Benchmark

环境：Go 1.27.1、pgx v5.11.0、PostgreSQL 17（本机临时 Docker 容器）、
Linux amd64、Intel i7-1355U，2026-09-08。

相同 SQL 和数据，每次操作扫描 1,000 行，每行一个含 64 个字符串的 text[] 列；
每行生成独立的 PDBField 消息。包含查询、网络、扫描器初始化和结果检查，
不包含建库和连接初始化。五次运行取中位数：

| 路径 | ms/op | B/op | allocs/op |
|---|---:|---:|---:|
| SQLScannerText（原有实现） | 44.132 | 10,313,449 | 339,020 |
| NativeBinary（本次实现） | 10.422 | 1,953,606 | 69,025 |

该场景耗时下降约 76%，分配内存下降约 81%，分配次数下降约 80%。
这里比较的是完整的“二进制数组查询 + 原生扫描”优化与原有路径，
不是把同一份二进制数据直接交给普通 sql.Scanner：pgx stdlib 不支持后者转字符串。

原始五次输出见 [benchmarks/rows-scan-go1.27.1.txt](benchmarks/rows-scan-go1.27.1.txt)。
本机耗时波动明显（原有路径 41.74–53.03 ms，新路径 10.16–11.07 ms）；
这组结果不能推广为所有查询、所有数组类型或动态消息都有相同比例的收益。

复现：

```bash
# 设置指向测试 PostgreSQL 的 PROTODB_SCAN_TEST_DSN
go test ./crud -run '^TestPGX' \
  -bench '^BenchmarkPGXArrayRowsColumnScanner$' -benchmem -count=5 -benchtime=1s

# 检查纯标量查询：优化选择器应保留原有标量接收器
go test ./crud -run '^$' -bench '^BenchmarkPGXRowsColumnScanner$' -benchmem
```

## 验证

- `go test ./... -race`，包含真实 PostgreSQL 集成测试；
- `go vet ./...`；
- Go 1.25.0 和 Go 1.26.0 下的 `go test ./...`；
- 文本/二进制 codec、整数范围、NULL/空值、缓冲区所有权、消息切换、
  JSON/类型不匹配/未知列回退、成对扫描、事务和预编译查询；
- 生成消息的字段编号定位，以及动态消息的通用数组接收。

不设置 `PROTODB_SCAN_TEST_DSN` 时，真实 PostgreSQL 测试和 benchmark 会明确跳过，
codec 与生成消息接收器单元测试仍会运行。
