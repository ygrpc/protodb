# protodb LLM Context

## Overview

`protodb` is a Go library that generates database schema and database CRUD/query operations from Protocol Buffer definitions. It uses `protoc-gen-go` and `protoc-gen-connect-go` to generate code. It supports Postgres, MySQL, and SQLite (database drivers are provided by the application).

## Core Concepts

### Proto Options

The library uses custom options defined in `protodb.proto` to control database behavior.

#### File Options (`protodb.pdbf`)

- `NameStyle` (string): Name style for message/field names (empty/"go" for Go style, "snake" for snake_case).
- `Comment` (repeated string): File-level comment.

#### Message Options (`protodb.pdbm`)

- `Comment` (repeated string): Table comment.
- `SQLPrepend` (repeated string): SQL before `CREATE TABLE`.
- `SQLAppend` (repeated string): SQL before the closing `)`.
- `SQLAppendsAfter` (repeated string): SQL after `)` but before `;`.
- `SQLAppendsEnd` (repeated string): SQL after `;`.
- `SQLMigrate` (repeated string): Reserved for migration SQL.
- `NotDB` (bool): If true, skip table generation.
- `MsgList` (int32): Control `{Msg}List` generation (0: auto, 1: always, 4: never).

#### Field Options (`protodb.pdb`)

- `NotDB` (bool): If true, skip column generation.
- `Primary` (bool): Primary key.
- `Unique` (bool): Unique constraint.
- `UniqueName` (string): Group name for composite unique constraints.
- `NotNull` (bool): `NOT NULL` constraint.
- `Reference` (string): Foreign key, e.g., `"table(field)"`.
- `DefaultValue` (string): Default value in DB.
- `SQLAppend` (repeated string): SQL fragments appended before the field comma.
- `SQLAppendsEnd` (repeated string): SQL fragments appended after the field comma.
- `NoUpdate` (bool): Ignore in UPDATE statements.
- `NoInsert` (bool): Ignore in INSERT statements.
- `SerialType` (int): 0: None, 2: SmallSerial, 4: Serial, 8: BigSerial.
- `DbType` (enum): `AutoMatch`, `BOOL`, `INT32`, `INT64`, `FLOAT`, `DOUBLE`, `TEXT`, `JSONB`, `UUID`, `TIMESTAMP`, `DATE`, `BYTEA`, `INET`, `UINT32`.
- `DbTypeStr` (string): Custom DB type string.
- `ZeroAsNull` (bool): Treat zero value as NULL.
- `Comment` (repeated string): Column comment.

### Runtime Architecture

#### Service Interface

The main service is `service.TconnectrpcProtoDbSrvHandlerImpl` which implements the `ProtoDbSrv` RPC interface.

```go
// TconnectrpcProtoDbSrvHandlerImpl
type TconnectrpcProtoDbSrvHandlerImpl struct {
 FnGetDb                   crud.TfnProtodbGetDb
 fnCrudPermissionMap       map[string]crud.TfnProtodbCrudPermission
 fnTableQueryPermissionMap map[string]crud.TfnTableQueryPermission
}
```

#### Hooks & Permissions

You must implement these functions to wire up the service:

1. **`TfnProtodbGetDb`**:

    ```go
    func(meta http.Header, schemaName string, tableName string, writable bool) (*sql.DB, error)
    ```

    - Returns the `sql.DB` connection to use.

2. **`TfnProtodbCrudPermission`**:

    ```go
    func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) error
    ```

    - Check if the user has permission to perform the CRUD operation.
    - `crudCode`: `INSERT`, `UPDATE`, `PARTIALUPDATE`, `DELETE`, `SELECTONE`.

3. **`TfnTableQueryPermission`**:

    ```go
    func(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (whereSqlStr string, whereSqlVals []any, err error)
    ```

    - Returns a WHERE clause fragment (and args) to enforce row-level security or filtering.

#### Msg Registration (`msgstore`)

CRUD/TableQuery need to resolve `TableName` to a concrete `proto.Message` via `msgstore`. Register messages at startup:

```go
msgstore.RegisterMsg("User", func(new bool) proto.Message {
    if new {
        return &User{}
    }
    return &User{}
})
```

If a message is not registered, requests fail with errors like `can not get proto msg ...`.

#### Custom Query Registration (`querystore`)

The `Query` streaming RPC uses `querystore.RegisterQuery(queryName, fn)` to build SQL and provide a `fnGetResultMsg` for scanning.

#### Permission Map Semantics (service layer)

- `fnCrudPermissionMap`: indexed by `TableName`. If the function is `nil` and `Code != SELECTONE`, the service returns permission denied.
- `fnTableQueryPermissionMap`: must contain a key for every table name used by `TableQuery` (the value can be `nil` to allow all rows).

#### Error Headers

When the request header contains `Ygrpc-Err-Header`, the service will also place error text into the response header `Ygrpc-Err` (optionally truncated by `Ygrpc-Err-Max`).

#### Database Dialect Detection

Dialect detection is based on the concrete `database/sql` driver type string (`reflect.TypeOf(db.Driver()).String()`):

- Postgres: `*pq.Driver` (lib/pq), `*stdlib.Driver` (pgx stdlib)
- MySQL: `*mysql.MySQLDriver` (go-sql-driver/mysql)
- SQLite: `*sqlite3.SQLiteDriver` (mattn/go-sqlite3), `*sqlite.Driver` (modernc.org/sqlite)

If the driver is unknown, the dialect falls back to `Unknown` and placeholders default to `?`.

### CRUD Operations (`crud` package)

- `Crud()`: Entry point for `INSERT`, `UPDATE`, `PARTIALUPDATE`, `DELETE`, `SELECTONE`.
- `TableQuery()`: Entry point for list/search queries.
- `Query()`: Entry point for custom SQL queries defined in `querystore`.

### Type Mapping (Postgres Example)

- `int32` -> `integer`
- `int64` -> `bigint`
- `string` -> `text`
- `bool` -> `boolean`
- `message` -> `jsonb`

## Usage Pattern

1. Define `.proto` with `protodb` options.
2. Generate code using `protoc` with `go` and `connect-go` plugins.
3. Implement `FnGetDb`, `FnCrudPermission`, `FnTableQueryPermission`.
4. Initialize `service.NewTconnectrpcProtoDbSrvHandlerImpl`.
5. Serve using `http.ListenAndServe`.
