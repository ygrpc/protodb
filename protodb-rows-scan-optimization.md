# protodb Rows Scan 性能优化方案

重构数据库扫描层，减少中间 scan dest 带来的类型转换、解包与反射开销，实现数据库值到 protobuf 字段的类型感知赋值。Phase 1 基于标准 `database/sql.Scanner`，Phase 2 记录进一步下沉到驱动层的设计草案。

## 问题分析

当前 `crud.DbScan2ProtoMsg` 的扫描流程存在以下浪费：

1. **中间 scan dest 转换**：每列预分配 `sql.Null*` 等接收器，扫描后还需要逐列解包并写入 protobuf。
2. **双重装箱**：`rows.Scan` 先把驱动原生值转成 `driver.Value` 再装箱进 `interface{}`。
3. **解箱 + 反射链**：兼容路径需要经过 `SetProtoMsgField` → `unwrapScanVal` → `pdbutil.SetField`。
4. **复杂字段二次解析**：`List/Map/Message` 字段先以 JSON/数组文本取出，再做 `json.Unmarshal` 或 `parsePGArrayLiteral`。

## 方案概述

### Phase 1：消除中间层，直接类型扫描（不依赖 Go 1.27，立即可做）

#### 1.1 重构 `DbScan2ProtoMsg` 的 dest 分配

不再使用统一的 `[]*interface{}`，改为为每列分配 `*protoFieldReceiver`（实现 `sql.Scanner` 接口）：

| Proto Kind | Dest 类型 | 说明 |
|---|---|---|
| 所有已知列 | `*protoFieldReceiver` | 实现 `sql.Scanner`，`Scan(src)` 内直接调用 `setProtoMsgFieldDirect` |
| 未知列 | `*any` | 回退兼容 |

`protoFieldReceiver` 在 `Scan(src)` 中根据 `FieldDescriptor.Kind()` 对 `src` 做类型感知转换（支持 `[]byte` → `string/int/float/bool` 等），然后直接写入 proto 字段。

#### 1.2 新增 `SetProtoMsgFieldDirect` 函数族

绕过 `pdbutil.SetField` 的反射，直接使用 `protoreflect` API：

```go
func setProtoMsgFieldString(msg proto.Message, fd protoreflect.FieldDescriptor, v string) error
func setProtoMsgFieldInt64(msg proto.Message, fd protoreflect.FieldDescriptor, v int64) error
func setProtoMsgFieldBool(msg proto.Message, fd protoreflect.FieldDescriptor, v bool) error
func setProtoMsgFieldFloat64(msg proto.Message, fd protoreflect.FieldDescriptor, v float64) error
func setProtoMsgFieldBytes(msg proto.Message, fd protoreflect.FieldDescriptor, v []byte) error
```

内部直接调用 `msg.ProtoReflect().Set(fd, protoreflect.ValueOfXxx(...))`，消除 `reflect` 开销。

#### 1.3 复用 Scan 上下文（多行场景）

`DbScan2ProtoMsg` 在循环扫描多行时，dest 数组对象本身可以复用，仅更新值，避免每行重复分配 dest 对象。

#### 1.4 收益与局限

**工作原理**：
- `rows.Scan` 在发现 dest 实现了 `sql.Scanner` 接口时，会调用其 `Scan(src)` 方法，把驱动解析后的值直接传入。
- `protoFieldReceiver.Scan` 内部直接调用 `setProtoMsgFieldDirect`，省去 `sql.NullString`/`sql.NullInt64` 等中间结构的转换和解包步骤。
- 每列的 dest 不再是 `*sql.NullString`，而是 `*protoFieldReceiver`。

- scan dest 与旧实现一样按列创建并跨行复用，但直接接收器减少了每行的转换和解包分配。
- 标量字段通常可直接写入 `protoreflect`，省去 `unwrapScanVal` 和 `pdbutil.SetField`；不支持的类型仍保留兼容回退。
- 继续使用标准 `database/sql` 接口，不要求底层驱动改造。
- `Scan(src any)` 仍接收装箱后的 `driver.Value`，Phase 1 不会实现零分配。

> **注意**：`database/sql` 的 `rows.Scan` 仍然会把驱动返回的 `driver.Value` 装箱进 `interface{}` 后再传给 `sql.Scanner.Scan(src any)`，因此 `driver.Value` → `interface{}` 的分配仍然存在。要彻底消除这一步，必须等 Go 1.27 的 `RowsColumnScanner.ScanColumn`（Phase 2）。

---

### Phase 2：Go 1.27 驱动层优化（`RowsColumnScanner`）

当 Go 1.27 发布且底层驱动（如 pgx）升级支持 `RowsColumnScanner` 后，进一步在驱动层实现零拷贝扫描。

#### 2.1 创建驱动包装器 `sqldb/protodbdriver`

实现 `driver.Driver` + `driver.Conn` + `driver.RowsColumnScanner`：

```go
package protodbdriver

type Driver struct{ underlying driver.Driver }

type rows struct {
    underlying driver.Rows
    colTypes   []driver.ColumnType // 缓存列类型元数据
}

func (r *rows) ScanColumn(scanCtx driver.ScanContext, index int, dest any) error {
    switch d := dest.(type) {
    case *protoFieldReceiver:
        // 利用底层驱动的列类型信息（OID、格式等）直接写入 proto 字段
        return d.receive(scanCtx, r.underlying, index)
    default:
        // 回退到标准类型转换
        v, err := r.readValue(index)
        if err != nil { return err }
        return sql.ConvertAssign(scanCtx, dest, v)
    }
}
```

#### 2.2 定义 `protoFieldReceiver` 接收器类型

在 `crud/dbscan.go` 中定义一组轻量接收器，封装 `proto.Message` + `protoreflect.FieldDescriptor`：

```go
type protoFieldReceiver struct {
    msg proto.Message
    fd  protoreflect.FieldDescriptor
}

// Go 1.27 之前：作为 sql.Scanner 使用，接收 driver.Value 后做标准转换
func (r *protoFieldReceiver) Scan(src any) error {
    return SetProtoMsgField(r.msg, r.fd, src)
}

// Go 1.27 + 驱动包装器：ScanColumn 直接调用，绕过 driver.Value 中间层
func (r *protoFieldReceiver) receive(scanCtx driver.ScanContext, rows driver.Rows, index int) error {
    // 根据 fd.Kind() 和数据库列的 OID/类型，直接读取并设置到 proto message
}
```

#### 2.3 类型感知的直接赋值示例

以 PostgreSQL `text[]` → `repeated string` 为例：

```go
func (r *protoFieldReceiver) receiveListString(scanCtx driver.ScanContext, rows driver.Rows, index int) error {
    // 传统路径：rows.Scan 返回 "{a,b,c}" 字符串，需 parsePGArrayLiteral
    // 新路径：利用 pgx 对 text[] 的原生解析，直接拿到 []string
    // 然后逐元素写入 protoreflect.List，跳过全部字符串序列化
}
```

同理，`jsonb` → `MessageKind` 可以直接利用底层驱动的 JSON 解析能力。

#### 2.4 用户接入方式

在 `sqldb` 包中提供辅助注册：

```go
func RegisterProtodbDriver(underlyingDriverName string) (protodbDriverName string, err error)
```

用户连接时：

```go
import "github.com/ygrpc/protodb/sqldb/protodbdriver"

_ = protodbdriver.RegisterProtodbDriver("pgx") // 注册 protodb-pgx

db, err := sql.Open("protodb-pgx", dsn)
```

**Phase 2 收益**：
- 驱动层直接操作 `protoreflect` API，跳过 `driver.Value` → `interface{}` → 反射 的全部中间层。
- PostgreSQL 数组可直接从二进制/文本格式解析为 `protoreflect.List`，跳过 `parsePGArrayLiteral` / `json.Unmarshal`。
- JSON/JSONB 字段可直接从驱动层解析为 proto message，跳过 `protojson.Unmarshal`。
- 对 MySQL/SQLite 同样适用：包装器可统一处理 JSON/数组类型的跨方言扫描。

---

### 关于底层驱动是否实现 RowsColumnScanner 的澄清

**Go 1.27 发布后，底层驱动（pgx、go-sql-driver/mysql、modernc.org/sqlite 等）不一定立即实现 `RowsColumnScanner`**，这取决于各驱动维护者的跟进节奏。

但 Go 1.27 的设计是**渐进式**的：
1. 如果底层驱动的 `Rows` 实现了 `RowsColumnScanner`，`database/sql` 自动调用 `ScanColumn`
2. 如果没有实现，回退到传统 `rows.Scan` 路径，行为与现在完全一致

在我们的架构中，**无论底层驱动是否跟进**，都能获得性能提升：

| 场景 | 路径 | 效果 |
|---|---|---|
| Go < 1.27 | `rows.Scan` → `*string`/`*int64` dest → `setProtoMsgFieldDirect` | 消除 `interface{}` 分配 + 反射 |
| Go >= 1.27，驱动**未实现** `RowsColumnScanner` | 同 Go < 1.27 路径 | Phase 1 收益仍在 |
| Go >= 1.27，驱动**已实现** `RowsColumnScanner` | `ScanColumn` → `ConvertAssign` → `protoFieldReceiver.Scan` | 标量字段跳过 `driver.Value` 装箱， Phase 1 收益 + 驱动层优化 |
| Go >= 1.27，使用 `protodbdriver` 包装器 | `ScanColumn` → 直接写入 `protoreflect` | 最大优化（可选，不依赖底层驱动） |

**`protodbdriver` 包装器的角色**：
- 它是一个**可选的增强层**，由 protodb 自己实现
- 即使底层驱动没有实现 `RowsColumnScanner`，我们的包装器也可以实现它
- 包装器内部通过底层驱动的扩展 API（如 pgx 的 `pgx.Rows`、mysql driver 的 `mysqlRows`）获取类型元数据和原始数据，直接写入 `protoreflect`
- 用户可自由选择：用标准驱动（Phase 1 优化）或用 `protodbdriver`（Phase 2 最大优化）

---

### Phase 3：Build Tag 区分与渐进式迁移

#### 3.1 Build Tag 隔离机制

使用 `//go:build` 标签将代码按 Go 版本隔离，确保同一套代码在旧版本上编译通过：

| 文件 | Build Tag | 说明 |
|---|---|---|
| `sqldb/protodbdriver/driver_go126.go` | `//go:build !go1.27` | Go < 1.27：不暴露 `RowsColumnScanner`，仅做驱动代理/占位 |
| `sqldb/protodbdriver/driver_go127.go` | `//go:build go1.27` | Go >= 1.27：实现完整 `RowsColumnScanner` + `ScanColumn` 路径 |
| `crud/dbscan_go126.go` | `//go:build !go1.27` | Go < 1.27：使用 Phase 1 优化后的标准 `rows.Scan` 路径 |
| `crud/dbscan_go127.go` | `//go:build go1.27` | Go >= 1.27：优先使用驱动包装的 `ScanColumn` 路径 |

#### 3.2 Go < 1.27 的折中路径

在 Go 1.23–1.26 上，Phase 1 的优化仍然有效：
- `DbScan2ProtoMsg` 仍然使用**类型感知的原生 dest 分配**（`*string`、`*int64` 等），消除 `interface{}` 开销。
- `setProtoMsgFieldDirect` 仍然通过 `protoreflect` API 直接写入字段，消除 `pdbutil.SetField` 反射。
- 但**无法绕过 `driver.Value` 中间层**，因为 `database/sql` 标准库仍然只能走 `rows.Scan(dest...)` → `driver.Value` → 转换的老路。

因此 Phase 1 的所有收益在旧版本上都能拿到；Phase 2 的驱动层零拷贝只能等 Go 1.27 才启用。

#### 3.3 兼容性保证

- **Phase 1 零侵入**：`DbScan2ProtoMsg`、`DbScan2ProtoMsgx2`、`SetProtoMsgField` 保留原有签名，内部根据条件选择最优路径。所有现有测试无需修改。
- **Phase 2 可选**：用户可选择使用 `protodbdriver` 包装驱动获得最大性能，也可继续使用标准驱动走 Phase 1 优化路径。
- **驱动回退**：`ScanColumn` 对不认识的 `dest` 类型返回 `driver.ErrSkip`，标准库自动回退到传统 `rows.Scan`。

#### 3.4 测试策略

1. 为 `setProtoMsgFieldDirect` 函数族编写单元测试，覆盖所有标量类型（bool/int32/int64/uint32/uint64/float32/float64/string/bytes/enum）。
2. 为重构后的 `DbScan2ProtoMsg` 编写 mock `sql.Rows` 测试，验证 dest 类型分配正确。
3. 为 `protodbdriver` 编写 mock driver 测试，验证 `ScanColumn` 的类型分发逻辑。
4. 保持现有集成测试通过（使用 `pgx` 标准路径验证兼容性）。

---

## 实施步骤

| Step | 任务 | 文件 | 说明 |
|---|---|---|---|
| 1 | 新增 `setProtoMsgFieldDirect` 函数族 | `crud/dbscan.go` | 覆盖所有标量类型的直接设置 |
| 2 | 重构 `DbScan2ProtoMsg` | `crud/dbscan.go` | 类型感知 dest 分配 + 直接字段设置 |
| 3 | 重构 `DbScan2ProtoMsgx2` | `crud/dbscan.go` | 同样消除 `[]*interface{}` |
| 4 | Benchmark 验证 | `crud/dbscan_bench_test.go` | 对比标量字段扫描性能 |
| 5 | 创建 `sqldb/protodbdriver` 包 | `sqldb/protodbdriver/driver.go` | 实现 `Driver` + `Conn` + `Rows` |
| 6 | 实现 `RowsColumnScanner.ScanColumn` | `sqldb/protodbdriver/rows.go` | 对 `protoFieldReceiver` 做类型感知写入 |
| 7 | 注册辅助函数 | `sqldb/protodbdriver/register.go` | `RegisterProtodbDriver` |
| 8 | 编写测试 | `sqldb/protodbdriver/*_test.go` | mock driver + 路径覆盖 |
| 9 | 文档更新 | `README.md` / `doc/` | 说明如何接入包装驱动 |

---

## 实测效果

`BenchmarkDbRowScannerScan` 使用相同的 `sqlmock` 数据对比预分配 scan dest 与 `protoFieldReceiver`，每次操作扫描 9 列、2 行。一次参考运行结果如下：

| 路径 | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `PreallocatedDest` | 18,315 | 2,358 | 37 |
| `ProtoFieldReceiver` | 15,234 | 2,217 | 26 |

运行命令：

```bash
go test ./crud -run '^$' -bench '^BenchmarkDbRowScannerScan$' -benchmem
```

具体耗时受机器和运行环境影响；这组 benchmark 主要用于持续比较两条相同扫描路径的分配数和相对性能。Phase 1 不承诺零分配，复杂数组和消息字段仍需文本或 JSON 解析。

> **注**：Phase 2 依赖于 Go 1.27 的发布以及底层驱动（pgx/mysql/sqlite）对 `RowsColumnScanner` 的实现跟进。在驱动未完全支持前，Phase 1 已经能带来显著的标量字段性能提升。
