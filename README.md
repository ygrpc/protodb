# protodb

**protodb** 是一个基于 [Protocol Buffers](https://protobuf.dev/) 的 Go 语言库，它能够根据您的 `.proto` 定义自动生成数据库表结构并提供强大的 CRUD (创建、读取、更新、删除) 功能。通过在 `.proto` 文件中添加自定义选项，您可以精确地控制数据库表的行为，从而将开发重心从繁琐的 SQL 编写转移到核心业务逻辑上。

## 核心特性

*   **Proto驱动的数据库建模:** 直接在 `.proto` 文件中定义您的数据模型和数据库表结构。
*   **自动化的 CRUD 操作:** 无需编写任何 SQL 语句，即可获得完整的增、删、改、查功能。
*   **灵活的查询接口:** 提供基于表和自定义查询的流式 RPC 接口。
*   **强大的自定义能力:** 通过自定义选项，可以轻松地定义主键、唯一键、索引、默认值、外键关联等。
*   **基于 `connectrpc` 的现代 RPC 框架:** 提供类型安全、高性能的 RPC 服务。
*   **权限控制:** 为 CRUD 和查询操作提供了灵活的权限控制钩子。

## 工作流程

1.  **环境准备:** 安装 `protoc` 编译器和相关的 Go 插件。
2.  **定义 Proto 文件:** 在 `.proto` 文件中定义您的消息 (message)，并使用 `protodb` 提供的自定义选项来注解您的消息和字段。
3.  **代码生成:** 运行项目提供的脚本（或直接使用 `protoc` 命令）来生成所有必要的 Go 代码，包括数据结构和服务接口。
4.  **实现服务:** 在您的 Go 代码中，实现 `protodb` 的服务接口，并提供数据库连接和权限控制逻辑。
5.  **运行和使用:** 启动您的 Go 服务，现在您可以通过 `connectrpc` 客户端来调用 `Crud`、`TableQuery` 和 `Query` 等方法，对数据库进行操作。

## 安装与代码生成

### 1. 前提条件

在生成代码之前，您需要安装以下工具：

*   **Protocol Buffer 编译器 (`protoc`)**: 请访问 [Protocol Buffer Compiler Installation](https://grpc.io/docs/protoc-installation/) 获取安装指南。

*   **Go 插件**: 安装 `protoc-gen-go` 和 `protoc-gen-connect-go`。

    ```bash
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
    ```

### 2. 代码生成

项目提供了一个示例脚本 `doc/build-proto.sh` 来演示如何从 `.proto` 文件生成 Go 代码。您可以参考或直接使用此脚本。

核心的生成命令如下：

```bash
#!/bin/bash

# 为项目核心的 protodb.proto 生成代码
protoc --proto_path=./ --go_out=paths=source_relative:./ \
  --connect-go_out=paths=source_relative:.
  protodb.proto

# 为文档示例中的 .proto 文件生成代码
protoc --proto_path=./doc --go_out=paths=source_relative:./doc \
  --connect-go_out=paths=source_relative:./doc \
  ./doc/db.proto ./doc/bfmap.rpc.proto
```

*   `--proto_path`: 指定 `.proto` 文件及其依赖的搜索路径。
*   `--go_out`: 指定 `protoc-gen-go` 插件的输出目录，用于生成基础的 Go 消息定义 (`.pb.go`)。
*   `--connect-go_out`: 指定 `protoc-gen-connect-go` 插件的输出目录，用于生成 ConnectRPC 的服务接口和客户端存根 (`.connect.go`)。
*   `paths=source_relative`: 这个选项确保生成的 Go 文件与源 `.proto` 文件位于同一目录中。

## 实现示例

### 1. 定义 `.proto` 文件

创建一个 `.proto` 文件 (例如 `mydatabase.proto`)，并定义您的消息。使用 `protodb` 的选项来指定数据库相关的属性。

```protobuf
syntax = "proto3";

package myapp;

import "protodb.proto";

option go_package = "github.com/my/app";

message User {
  option (protodb.pdbm) = {
    Comment: ["用户表"]
  };

  int64 user_id = 1 [(protodb.pdb).Primary = true, (protodb.pdb).SerialType = 8];
  string username = 2 [(protodb.pdb).Unique = true, (protodb.pdb).NotNull = true];
  string email = 3 [(protodb.pdb).Unique = true];
  int64 create_time = 4 [(protodb.pdb).NoUpdate = true];
}
```

### 2. 实现服务

在您的 Go 代码中，创建一个 `connectrpc` 服务处理器，并实现 `protodb` 的接口。

```go
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/crud"
	"github.com/ygrpc/protodb/service"
	"google.golang.org/protobuf/proto"
	// 您的数据库驱动
)

func main() {
	// 1. 数据库连接
	db, err := sql.Open("your_driver_name", "your_connection_string")
	if err != nil {
		log.Fatal(err)
	}

	// 2. 权限控制 (示例：允许所有操作)
	fnCrudPermission := func(ctx context.Context, meta http.Header, db *sql.DB, code protodb.CrudReqCode, msg proto.Message) error {
		return nil
	}
	fnTableQueryPermission := func(ctx context.Context, meta http.Header, db *sql.DB, req *protodb.TableQueryReq) error {
		return nil
	}

	// 3. 创建 protodb 服务处理器
	protoDbSrv := service.NewTconnectrpcProtoDbSrvHandlerImpl(
		func(ctx context.Context, meta http.Header, code protodb.CrudReqCode, msg proto.Message) (*sql.DB, error) {
			return db, nil
		},
		map[string]crud.TfnProtodbCrudPermission{
			"User": fnCrudPermission,
		},
		map[string]crud.TfnTableQueryPermission{
			"User": fnTableQueryPermission,
		},
	)

	// 4. 启动 HTTP 服务
	mux := http.NewServeMux()
	path, handler := protodb.NewProtoDbSrvHandler(protoDbSrv)
	mux.Handle(path, handler)
	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", mux)
}
```

## 一个更完整的示例

让我们通过一个更真实的例子来看看 `protodb` 的威力。假设我们正在构建一个名为 `bfmap` 的应用。

### 1. 定义数据库模式 (`db.proto`)

我们首先在 `db.proto` 文件中定义所有的数据库表。`protodb` 的选项让我们能够精确地描述表结构。

以 `DbUser` 表为例，它展示了主键、唯一键、外键、默认值和自定义 SQL 等多种用法。

```protobuf
// doc/db.proto

syntax = "proto3";

package dbproto;

import "protodb.proto";

option go_package = "bfmap/dbproto";

// ... 其他消息定义 ...

message DbUser {
  option (protodb.pdbm).SQLAppendsEnd = "CREATE INDEX IF NOT EXISTS idx_DbUser_Creater ON DbUser (Creater);";
  // rid uuidv7
  string Rid = 1 [ (protodb.pdb) = {Primary : true, DbType : UUID} ];
  // org, 外键关联到 DbOrg 表，并设置级联删除
  string Org = 2 [
    (protodb.pdb) = {DbType : UUID, Reference : "DbOrg(Rid)", ZeroAsNull : true, SQLAppend: "ON DELETE CASCADE"}
  ];
  // name, 设置为唯一且非空
  string Name = 3 [ (protodb.pdb) = {Unique: true, NotNull: true} ];
  // nickname
  string Nickname = 4;
  // email
  string Email = 5;
  // phone
  string Phone = 6;
  // password, base64(sha256(Name+Password))
  string Password = 7;
  // create time
  int64 CreateTime = 8 [ (protodb.pdb) = {DbType : TIMESTAMP, DefaultValue: "0"} ];
  // creater uuid
  string Creater = 9 [ (protodb.pdb) = {DbType : UUID}];
  // note
  string Note = 10;
  // setting, 使用 JSONB 类型存储
  string Setting = 11 [ (protodb.pdb) = {DbType : JSONB, DefaultValue : "{}"} ];
  // disabled, 设置布尔类型的默认值
  bool Disabled = 12 [ (protodb.pdb) = {DefaultValue : "false"} ];
}

// ... 其他消息定义 ...
```

### 2. 定义 RPC 服务 (`bfmap.rpc.proto`)

接下来，我们在 `bfmap.rpc.proto` 中定义应用的 RPC 接口。关键在于，我们可以直接导入并复用 `db.proto` 中定义的消息。这确保了我们的 API 和数据库之间的数据模型完全一致。

```protobuf
// doc/bfmap.rpc.proto

syntax = "proto3";

package rpc;

option go_package = "bfmap/rpc";

// 直接导入数据库模式定义
import "db.proto";

// ...

message CreateUserReq {
  dbproto.DbUser User = 1;
  dbproto.DbUserPrivilege UserPrivilege = 2;
}

// bfmap rpc service
// 需要在 header 中添加 "Session-Id"
service bfmap {
  // ... 其他 RPC 方法 ...

  // CreateUser 方法直接使用了 db.proto 中的 DbUser 消息
  rpc CreateUser(CreateUserReq) returns (Common);

  // ... 其他 RPC 方法 ...
}
```

### 3. 结合与实现

1.  **代码生成**: 运行 `doc/build-proto.sh` 脚本后，`protodb` 会为 `db.proto` 中的每个消息生成对应的 Go struct 和 CRUD 函数。同时，`connectrpc` 的插件会为 `bfmap.rpc.proto` 生成服务接口和客户端存根。
2.  **服务实现**: 在后端服务中，我们实现 `bfmap` 服务。当 `CreateUser` 请求到达时，我们可以从 `CreateUserReq` 中获取 `DbUser` 对象。
3.  **数据库操作**: 我们可以直接调用 `protodb` 自动生成的 `crud.Crud()` 函数，将 `DbUser` 对象作为参数传递进去，`protodb` 会自动处理数据库的插入操作。

这个流程极大地简化了开发。您只需要在 `.proto` 文件中维护您的数据模型，`protodb` 会负责处理所有底层的数据库交互，让您能更专注于业务逻辑的实现。

## Proto 定义详解

`protodb` 提供了丰富的自定义选项，让您可以精细地控制数据库表的生成和行为。

### `PDBMsg` (消息选项)

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| `Comment` | `repeated string` | 表的注释。 |
| `SQLPrepend` | `repeated string` | 在 `CREATE TABLE` 语句之前添加的 SQL。 |
| `SQLAppend` | `repeated string` | 在 `CREATE TABLE` 语句的 `)` 之前添加的 SQL。 |
| `SQLAppendsAfter` | `repeated string` | 在 `CREATE TABLE` 语句的 `)` 之后、`;` 之前添加的 SQL。 |
| `SQLAppendsEnd` | `repeated string` | 在 `CREATE TABLE` 语句的 `;` 之后添加的 SQL。 |
| `NotDB` | `bool` | 如果为 `true`，则不为此消息生成数据库表。 |

### `PDBField` (字段选项)

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| `NotDB` | `bool` | 如果为 `true`，则不在数据库表中创建此字段。 |
| `Primary` | `bool` | 将此字段标记为主键。 |
| `Unique` | `bool` | 将此字段标记为唯一键。 |
| `UniqueName` | `string` | 用于组合唯一键的名称。 |
| `NotNull` | `bool` | 将此字段标记为 `NOT NULL`。 |
| `Reference` | `string` | 定义外键引用，例如 `"other_table(other_field)"`。 |
| `DefaultValue` | `string` | 字段的默认值。 |
| `NoUpdate` | `bool` | 在更新操作中忽略此字段 (例如，创建时间)。 |
| `NoInsert` | `bool` | 在插入操作中忽略此字段 (例如，数据库有默认值)。 |
| `SerialType` | `int32` | 定义自增类型 (2: `smallint`, 4: `integer`, 8: `bigint`)。 |
| `DbType` | `FieldDbType` | 指定字段的数据库类型 (例如 `TEXT`, `JSONB`, `UUID`)。 |
| `DbTypeStr` | `string` | 使用自定义的数据库类型字符串。 |
| `ZeroAsNull` | `bool` | 将字段的零值 (例如 `0`, `""`) 在插入/更新时视作 `NULL`。 |
| `Comment` | `repeated string` | 字段的注释。 |

## 贡献

欢迎对 `protodb` 做出贡献！如果您有任何问题、建议或改进，请随时提交 Issue 或 Pull Request。