# protodb Rows Scan 性能优化方案

利用 Go 1.27 的 `driver.RowsColumnScanner` 与 `sql.ConvertAssign`，重构数据库扫描层，消除 `[]*interface{}` 中间层带来的 N 次堆分配、双重类型转换与反射开销，实现驱动层到 protobuf 字段的直接类型感知赋值。

## 问题分析

当前 `crud.DbScan2ProtoMsg` 的扫描流程存在以下浪费：

1. **N 次 `interface{}` 堆分配**：每列 `new(interface{})`，每行重复。
2. **双重装箱**：`rows.Scan` 先把驱动原生值转成 `driver.Value` 再装箱进 `interface{}`。
3. **解箱 + 反射链**：`SetProtoMsgField` → `unwrapScanVal` → `pdbutil.SetField`（大量 `reflect` 操作）。
4. **复杂字段二次解析**：`List/Map/Message` 字段先以 JSON/数组文本取出，再做 `json.Unmarshal` 或 `parsePGArrayLiteral`。

## 方案概述

### Phase 1：消除中间层，直接类型扫描（不依赖 Go 1.27，立即可做）

#### 1.1 重构 `DbScan2ProtoMsg` 的 dest 分配

不再使用统一的 `[]*interface{}`，改为根据 `protoreflect.FieldDescriptor.Kind()` 为每列选择最优的原生 dest 类型：

| Proto Kind | Dest 类型 | 说明 |
|---|---|---|
| `StringKind` | `*string` | 直接接收文本 |
| `Int32/64/Sint/Sfixed/Enum` | `*int64` | 数据库存 bigint/integer |
| `Uint32/64/Fixed` | `*int64` | 数据库通常存为有符号，后续转换 |
| `BoolKind` | `*bool` | 直接接收 boolean |
| `FloatKind`/`DoubleKind` | `*float64` | 直接接收 real/double |
| `BytesKind` | `*[]byte` | 直接接收 bytea/blob |
| `MessageKind` / `List` / `Map` | `*string` 或 `*[]byte` | JSON/数组文本，后续解析 |
| 未知列 | `*any` | 回退兼容 |

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

**Phase 1 收益**：
- 消除每行每列的 `interface{}` 堆分配。
- 消除 `unwrapScanVal` 的间接层。
- 消除标量字段的 `pdbutil.SetField` 反射开销。
- 完全兼容现有驱动，无外部依赖变化。

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

#### 3.2 测试策略

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

## 预期效果

| 优化项 | 当前开销 | Phase 1 后 | Phase 2 后 |
|---|---|---|---|
| 标量字段堆分配 | N `interface{}` / row | ≈ 0（复用栈/堆 string/int64 等） | ≈ 0 |
| 标量字段类型转换 | `driver.Value` → `interface{}` → 反射 | `driver.Value` → 直接 `protoreflect.Set` | 驱动原生 → 直接 `protoreflect.Set` |
| 数组字段解析 | JSON/文本序列化 + 反序列化 | 同上（仍走文本） | 驱动层直接解析为 `protoreflect.List` |
| JSON 字段解析 | `protojson.Unmarshal` | 同上 | 驱动层直接解析为 `proto.Message` |
| 整体拷贝次数 | 2-3 次 | 2 次 | 1 次 |

> **注**：Phase 2 依赖于 Go 1.27 的发布以及底层驱动（pgx/mysql/sqlite）对 `RowsColumnScanner` 的实现跟进。在驱动未完全支持前，Phase 1 已经能带来显著的标量字段性能提升。
