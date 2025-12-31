# protodb

**protodb** 是一个基于 [Protocol Buffers](https://protobuf.dev/) 的强大 Go 语言库，旨在简化数据库开发流程。它通过直接在 `.proto` 文件中定义数据模型和数据库选项，自动为您生成完整的数据库表结构和高性能的 CRUD（增删改查）代码。

**核心理念：让 Protocol Buffers 成为您的单一事实来源 (Single Source of Truth)。**

---

## 🔥 核心特性

* **Proto 驱动开发**: 直接在 `.proto` 中定义表结构、索引、约束和默认值。
* **全自动 CRUD**: 自动生成 Insert, Update, PartialUpdate, Delete, SelectOne 等常用操作代码，告别手写 SQL。
* **高级查询支持**: 提供基于 RPC 的流式查询接口 (`TableQuery`, `Query`)，支持灵活的过滤和分页。
* **多数据库支持**: 兼容 **PostgreSQL**, **MySQL**, **SQLite**。
* **ConnectRPC 集成**: 基于现代的 `connect-go` 框架，提供类型安全、高性能的 HTTP/2 RPC 服务。
* **细粒度权限控制**: 灵活的 Hook 机制，允许您精确控制每个表、每个操作的访问权限。
* **丰富的字段映射**: 支持 JSONB, UUID, IP Address, Timestamp 等高级数据类型。

---

## 🚀 快速开始

### 1. 环境准备

确保您已安装 Go 和 Protocol Buffer 编译器 (`protoc`)。

数据库驱动请按需在您的应用侧引入（例如 Postgres 可用 `github.com/lib/pq` 或 `github.com/jackc/pgx/v5/stdlib`；MySQL 可用 `github.com/go-sql-driver/mysql`；SQLite 可用 `github.com/mattn/go-sqlite3` 或 `modernc.org/sqlite`）。

```bash
# 安装 Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
```

### 2. 定义数据模型

在您的 `.proto` 文件中导入 `protodb.proto`，并使用我们提供的选项来注解您的消息。

```protobuf
syntax = "proto3";
package myapp;

import "protodb.proto"; // 导入 protodb 定义
option go_package = "github.com/my/app";

message User {
  // 表级选项：添加注释
  option (protodb.pdbm) = {
    Comment: ["用户表"]
  };

  // 字段级选项：主键，自动生成 ID (Snowflake/UUID 由应用层或 DB 层处理，这里演示自增或 UUID)
  // 这里的 SerialType=8 对应 Postgres 的 bigserial
  int64 id = 1 [(protodb.pdb).Primary = true, (protodb.pdb).SerialType = 8];

  // 唯一索引，非空约束
  string username = 2 [(protodb.pdb).Unique = true, (protodb.pdb).NotNull = true];

  // 映射为 JSONB 类型，设置默认值
  string settings = 3 [(protodb.pdb).DbType = JSONB, (protodb.pdb).DefaultValue = "{}"];
  
  // 自动忽略更新（如创建时间）
  int64 created_at = 4 [(protodb.pdb).NoUpdate = true];
}
```

### 3. 生成代码

编写脚本生成 Go 代码。

```bash
#!/bin/bash
protoc --proto_path=. --go_out=paths=source_relative:. \
  --connect-go_out=paths=source_relative:. \
  your_file.proto
```

### 4. 实现服务

在您的 Go 代码中实现 `ProtoDbSrv` 服务。你需要提供两个核心的回调函数：`GetDb` (获取数据库连接) 和 `Permission` (权限检查)。

注意：`crud` 包仅提供 ORM/SQL 能力（如 `DbInsert`/`DbUpdate` 等）。RPC Handler、广播器以及相关的权限回调类型位于 `service` 包。

```go
package main

import (
    "database/sql"
    "net/http"
    "google.golang.org/protobuf/proto"
    "github.com/ygrpc/protodb"
    "github.com/ygrpc/protodb/msgstore"
    "github.com/ygrpc/protodb/service"
    "github.com/ygrpc/protodb/sqldb"
    _ "github.com/lib/pq" // Postgres 驱动
)

func main() {
    // 1. 初始化数据库
    db, _ := sql.Open("postgres", "...")

    // 1.1 注册 proto message（TableName -> proto.Message）
    // Crud/TableQuery/SelectOne 会通过 TableName 从 msgstore 里取消息类型
    msgstore.RegisterMsg("User", func(new bool) proto.Message {
        if new {
            return &User{}
        }
        return &User{}
    })

    // 2. 定义获取数据库连接的函数
    fnGetDb := func(meta http.Header, schema, table string, writable bool) (sqldb.DB, error) {
        return db, nil // 在实际场景中，可以根据 table 做分库分表
    }

    // 3. 定义 CRUD 权限控制 (示例：允许所有)
    fnCrudPerm := func(meta http.Header, schema string, code protodb.CrudReqCode, db sqldb.DB, msg proto.Message) error {
        return nil // 返回 error 则拒绝操作
    }
    
    // 4. 定义查询权限控制 (示例：只查询自己的数据)
    // 返回 SQL 过滤条件 (WHERE ...)
    fnQueryPerm := func(meta http.Header, schema, table string, db sqldb.DB, msg proto.Message) (string, []any, error) {
        // e.g. return "user_id = $1", []any{currentUserId}, nil
        return "", nil, nil
    }

    // 5. 创建并启动服务
    srv := service.NewTconnectrpcProtoDbSrvHandlerImpl(
        fnGetDb,
        map[string]service.TfnProtodbCrudPermission{"User": fnCrudPerm},     // 针对 User 表的写权限
        map[string]service.TfnTableQueryPermission{"User": fnQueryPerm},   // 针对 User 表的读权限
    )
    
    mux := http.NewServeMux()
    path, handler := protodb.NewProtoDbSrvHandler(srv)
    mux.Handle(path, handler)
    http.ListenAndServe(":8080", mux)
}
```

---

## 📖 详细配置手册

### 表选项 (Message Options)

使用 `option (protodb.pdbm) = { ... };` 设置。

| 选项名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `Comment` | `[]string` | 数据库表的注释。 |
| `NotDB` | `bool` | 设为 `true` 则不生成该表。 |
| `SQLPrepend` | `[]string` | 在 `CREATE TABLE` **之前** 执行的 SQL。 |
| `SQLAppend` | `[]string` | 在字段定义结束符 `)` **之前** 插入的 SQL（常用于定义联合主键等）。 |
| `SQLAppendsAfter` | `[]string` | 在 `)` **之后**、`;` **之前** 追加的 SQL。 |
| `SQLAppendsEnd` | `[]string` | 在 `CREATE TABLE` 语句结束符 `;` **之后** 执行的 SQL（常用于创建索引）。 |
| `MsgList` | `int32` | 控制 `{Msg}List` 消息生成策略 (0:自动, 1:强制生成, 4:不生成)。 |
| `SQLMigrate` | `[]string` | 预留：用于迁移的 SQL（当前仓库主要用于建表 SQL 生成，迁移需自行组织调用）。 |

### 文件选项 (File Options)

使用 `option (protodb.pdbf) = { ... };` 设置。

| 选项名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `NameStyle` | `string` | 名称风格：默认空/`"go"` 为 Go 风格（如 `UserName`），`"snake"` 为下划线（如 `user_name`）。 |
| `Comment` | `[]string` | 文件级注释（用于说明/生成 SQL 注释等场景）。 |

### 字段选项 (Field Options)

使用 `[(protodb.pdb) = { ... }];` 设置。

| 选项名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `NotDB` | `bool` | 设为 `true` 则不生成该字段列。 |
| `Primary` | `bool` | 设为主键。 |
| `Unique` | `bool` | 设为唯一索引。 |
| `UniqueName` | `string` | 联合唯一索引的组名。相同组名的字段会组成一个联合唯一索引。 |
| `NotNull` | `bool` | 添加 `NOT NULL` 约束。 |
| `DefaultValue` | `string` | 数据库字段的默认值 (SQL 语法)。 |
| `Reference` | `string` | 外键约束，格式：`"other_table(column)"`。 |
| `SQLAppend` | `[]string` | 在字段定义的 `,` **之前**追加 SQL 片段。 |
| `SQLAppendsEnd` | `[]string` | 在字段定义的 `,` **之后**追加 SQL 片段。 |
| `NoUpdate` | `bool` | 更新记录时忽略此字段 (例如 `create_time`)。 |
| `NoInsert` | `bool` | 插入记录时忽略此字段 (例如使用数据库自增或默认值)。 |
| `SerialType` | `int` | 自增类型映射: `2`=SmallSerial, `4`=Serial, `8`=BigSerial。 |
| `DbType` | `Enum` | 强制指定数据库类型（如 `JSONB`, `UUID`, `INET`, `TEXT`, `BOOL` 等；默认 `AutoMatch`）。 |
| `DbTypeStr` | `string` | 直接指定自定义 DB 类型字符串（优先级高于 `DbType`）。 |
| `ZeroAsNull` | `bool` | 插入/更新时，如果 Go 结构体中是零值，则写入数据库 `NULL`。 |
| `Comment` | `[]string` | 字段注释（在生成 SQL 且开启 comment 输出时生效）。 |

### 类型映射表 (Postgres 示例)

| Proto 类型 | 默认 DB 类型 | 可选 DbType | 说明 |
| :--- | :--- | :--- | :--- |
| `bool` | `boolean` | - | - |
| `int32` | `integer` | `SerialType=4` -> `serial` | - |
| `int64` | `bigint` | `SerialType=8` -> `bigserial` | - |
| `string` | `text` | `JSONB`, `UUID`, `INET`, `TEXT` | 默认 text，可映射为高级类型 |
| `bytes` | `bytea` | - | - |
| `float` | `real` | - | - |
| `double` | `double precision` | - | - |
| `message` | `jsonb` | - | 嵌套消息自动存储为 JSONB |

---

## 🛠️ 进阶使用

### 1. 复杂查询 (TableQuery)

`TableQuery` RPC 允许客户端灵活查询：

* **指定返回列**: 仅获取需要的字段。
* **Where 过滤**: 支持 `Field == Value` 的简单过滤。
* **Where2 高级过滤**: 支持 `WOP_GT` (>), `WOP_LT` (<), `WOP_LIKE` (Like) 等操作符。
* **分页**: `Limit` 和 `Offset`。

注意：当使用 `Where2` 时，需要同时填充 `Where2Operator`，且两者长度必须一致。

### 2. 自定义 SQL 查询 (Query)

对于 `protodb` 自动生成的 CRUD 无法满足的复杂场景（如多表 Join），您可以在 `querystore` 中注册自定义 SQL，并通过 `Query` RPC 调用。客户端只需传递参数，依然保持类型安全。

### 3. 表结构自动迁移

`protodb` 包含一些能够根据 Proto 定义生成 `CREATE TABLE` 语句的逻辑，这使得从定义到部署非常顺滑。您可以编写脚本调用 `pdbutil` 相关函数来输出 Schema SQL。

### 4. 事务支持 (Transaction Support)

`protodb` 支持在事务中执行多个原子性的数据库操作。这对于金融、订单等严肃的业务系统至关重要。

#### DB 接口

所有 CRUD 函数现在都接受 `sqldb.DB` 接口，该接口同时被 `*sql.DB` 和 `*sql.Tx` 实现：

```go
// DB 定义了 *sql.DB 和 *sql.Tx 的通用方法
type DB interface {
    Exec(query string, args ...any) (sql.Result, error)
    Query(query string, args ...any) (*sql.Rows, error)
    QueryRow(query string, args ...any) *sql.Row
}
    // ... 以及 Context 版本的方法
}
```

#### 基本事务用法

```go
import (
    "database/sql"
    "github.com/ygrpc/protodb/crud"
    "github.com/ygrpc/protodb/sqldb"
)

// 在事务中执行多个操作
func CreateOrderWithItems(db *sql.DB, order *Order, items []*OrderItem) error {
    // 开始事务
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    
    // 获取数据库方言（在事务开始前）
    dialect := sqldb.GetDBDialect(db)
    
    // 创建带方言信息的 executor
    executor := sqldb.NewTxWithDialectType(tx, dialect)
    
    // 插入订单
    _, err = crud.DbInsert(executor, order, 0, "")
    if err != nil {
        tx.Rollback()
        return err
    }
    
    // 插入订单项（与订单在同一事务中）
    for _, item := range items {
        _, err = crud.DbInsert(executor, item, 0, "")
        if err != nil {
            tx.Rollback() // 回滚整个事务
            return err
        }
    }
    
    // 提交事务
    return tx.Commit()
}
```

#### 服务层事务封装示例

```go
// RunInTransaction 提供一个通用的事务封装
func RunInTransaction(db *sql.DB, fn func(tx sqldb.DB) error) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    
    dialect := sqldb.GetDBDialect(db)
    executor := sqldb.NewTxWithDialectType(tx, dialect)
    
    if err := fn(executor); err != nil {
        tx.Rollback()
        return err
    }
    
    return tx.Commit()
}

// 使用示例
err := RunInTransaction(db, func(tx sqldb.DB) error {
    // 所有操作在同一事务中执行
    _, err := crud.DbInsert(tx, order, 0, "")
    if err != nil {
        return err
    }
    
    _, err = crud.DbUpdate(tx, inventory, 0, "")
    return err
})
```

#### 兼容性说明

* **向后兼容**: 现有使用 `*sql.DB` 的代码无需修改，可以直接继续工作
* **新代码建议**: 使用 `sqldb.DB` 接口以获得事务支持
* **注意事项**: 使用 `*sql.Tx` 时，需要用 `sqldb.DBWithDialect` 包装以保留数据库方言信息

---

## 🤝 贡献

欢迎提交 Issue 和 PR！
