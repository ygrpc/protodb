# protodb LLM Context

## Overview

`protodb` is a Go library that generates database schema and CRUD operations from Protocol Buffer definitions. It uses `protoc-gen-go` and `protoc-gen-connect-go` to generate code. It supports Postgres, MySQL, and SQLite.

## Core Concepts

### Proto Options

The library uses custom options defined in `protodb.proto` to control database behavior.

#### Message Options (`protodb.pdbm`)

- `Comment` (repeated string): Table comment.
- `SQLPrepend` (repeated string): SQL before `CREATE TABLE`.
- `SQLAppend` (repeated string): SQL before the closing `)`.
- `SQLAppendsAfter` (repeated string): SQL after `)` but before `;`.
- `SQLAppendsEnd` (repeated string): SQL after `;`.
- `NotDB` (bool): If true, skip table generation.
- `MsgList` (int32): Control `{Msg}List` generation (0: auto, 1: always, 4: never).

#### Field Options (`protodb.pdb`)

- `Primary` (bool): Primary key.
- `Unique` (bool): Unique constraint.
- `UniqueName` (string): Group name for composite unique constraints.
- `NotNull` (bool): `NOT NULL` constraint.
- `Reference` (string): Foreign key, e.g., `"table(field)"`.
- `DefaultValue` (string): Default value in DB.
- `NoUpdate` (bool): Ignore in UPDATE statements.
- `NoInsert` (bool): Ignore in INSERT statements.
- `SerialType` (int): 0: None, 2: SmallSerial, 4: Serial, 8: BigSerial.
- `DbType` (enum): `BOOL`, `INT32`, `INT64`, `FLOAT`, `DOUBLE`, `TEXT`, `JSONB`, `UUID`, `TIMESTAMP`, `DATE`, `BYTEA`, `INET`.
- `DbTypeStr` (string): Custom DB type string.
- `ZeroAsNull` (bool): Treat zero value as NULL.

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
    - `crudCode`: `INSERT`, `UPDATE`, `DELETE`, `SELECTONE`.

3. **`TfnTableQueryPermission`**:

    ```go
    func(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (whereSqlStr string, whereSqlVals []any, err error)
    ```

    - Returns a WHERE clause fragment (and args) to enforce row-level security or filtering.

### CRUD Operations (`crud` package)

- `Crud()`: Entry point for Insert, Update, Delete, SelectOne.
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
