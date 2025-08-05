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

1.  **定义 Proto 文件:** 在 `.proto` 文件中定义您的消息 (message)，并使用 `protodb` 提供的自定义选项来注解您的消息和字段。
2.  **代码生成:** 运行 `build.sh` 脚本，它会调用 `protoc` 编译器以及相关的 Go 插件，生成所有必要的 Go 代码，包括数据结构和服务接口。
3.  **实现服务:** 在您的 Go 代码中，实现 `protodb` 的服务接口，并提供数据库连接和权限控制逻辑。
4.  **运行和使用:** 启动您的 Go 服务，现在您可以通过 `connectrpc` 客户端来调用 `Crud`、`TableQuery` 和 `Query` 等方法，对数据库进行操作。

## 快速开始

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

### 2. 生成 Go 代码

运行 `build.sh` 脚本来生成 Go 代码：

```bash
./build.sh
```

### 3. 实现服务

在您的 Go 代码中，创建一个 `connectrpc` 服务处理器，并实现 `protodb` 的接口。

```go
package main

import (
	"context"
	"database/sql"
	"net/http"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/crud"
	"github.com/ygrpc/protodb/service"
	// 您的数据库驱动
)

func main() {
	// 1. 数据库连接
	db, err := sql.Open("your_driver_name", "your_connection_string")
	if err != nil {
		log.Fatal(err)
	}

	// 2. 权限控制 (示例：允许所有操作)
	fnCrudPermission := func(ctx context.Context, meta http.Header, db sql.DB, code protodb.CrudReqCode, msg proto.Message) error {
		return nil
	}
	fnTableQueryPermission := func(ctx context.Context, meta http.Header, db sql.DB, req *protodb.TableQueryReq) error {
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
	http.ListenAndServe(":8080", mux)
}
```

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