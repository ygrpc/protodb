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
 FnGetDb                   service.TfnProtodbGetDb
 fnCrudPermissionMap       map[string]service.TfnProtodbCrudPermission
 fnTableQueryPermissionMap map[string]service.TfnTableQueryPermission
}
```

#### Hooks & Permissions

You must implement these functions to wire up the service:

1. **`TfnProtodbGetDb`** (recommended):

    ```go
    func(meta http.Header, schemaName string, tableName string, writable bool) (sqldb.DB, error)
    ```

    - Returns a `sqldb.DB` which can be `*sql.DB`, `*sql.Tx`, or `*sqldb.DBWithDialect`.
    - Use this for transaction support.

2. **`TfnProtodbCrudPermission`** (recommended):

    ```go
    func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DB, dbmsg proto.Message) error
    ```

    - Same as above but accepts `DB` for transaction support.

3. **`TfnTableQueryPermission`** (recommended):

    ```go
    func(meta http.Header, schemaName string, tableName string, db sqldb.DB, dbmsg proto.Message) (whereSqlStr string, whereSqlVals []any, err error)
    ```

    - Same as above but accepts `DB` for transaction support.

#### Transaction Support (`sqldb.DB`)

The `DB` interface (`sqldb/executor.go`) abstracts the common methods of `*sql.DB` and `*sql.Tx`, enabling:

- **Single operations** using `*sql.DB` directly (auto-commit each operation)
- **Multiple atomic operations** using `*sql.Tx` for transactions

```go
// DB interface methods:
type DB interface {
    Exec(query string, args ...any) (sql.Result, error)
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    Query(query string, args ...any) (*sql.Rows, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRow(query string, args ...any) *sql.Row
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
    Prepare(query string) (*sql.Stmt, error)
    PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
```

**Usage Examples:**

```go
// Using with *sql.DB (auto-commit each operation)
var executor sqldb.DB = db
crud.DbInsert(executor, msg, lastFieldNo, schema)

// Using with *sql.Tx (atomic transaction)
tx, _ := db.Begin()
dialect := sqldb.GetDBDialect(db) // Get dialect before transaction
executor := sqldb.NewTxWithDialectType(tx, dialect)
crud.DbInsert(executor, msg1, lastFieldNo, schema)
crud.DbUpdate(executor, msg2, lastFieldNo, schema)
tx.Commit() // or tx.Rollback() on error
```

**Important:** When using `*sql.Tx`, you should wrap it with `sqldb.DBWithDialect` to preserve dialect information, since `*sql.Tx` doesn't expose the underlying driver type.

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

**Note:** For `*sql.Tx`, use `sqldb.GetExecutorDialect()` or wrap the transaction with `sqldb.DBWithDialect` to preserve dialect information.

### CRUD Operations (`crud` package)

- `DbInsert`, `DbUpdate`, `DbDelete`, `DbSelectOne`, etc: ORM operations.
- All ORM functions accept `sqldb.DB` instead of `*sql.DB`, enabling transaction support.

### RPC Orchestration (`service` package)

- `HandleCrud()`: Entry point for `INSERT`, `UPDATE`, `PARTIALUPDATE`, `DELETE`, `SELECTONE`.
- `HandleTableQuery()`: Entry point for list/search queries.
- `HandleQuery()`: Entry point for custom SQL queries defined in `querystore`.

All CRUD functions (`DbInsert`, `DbUpdate`, `DbDelete`, `DbSelectOne`, etc.) now accept `sqldb.DB` instead of `*sql.DB`, enabling transaction support.

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

## Transaction Example

```go
// Service-level transaction handling
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

// Usage
err := RunInTransaction(db, func(tx sqldb.DB) error {
    // Insert order
    _, err := crud.DbInsert(tx, orderMsg, 0, "")
    if err != nil {
        return err
    }
    
    // Insert order items (atomic with order)
    for _, item := range orderItems {
        _, err := crud.DbInsert(tx, item, 0, "")
        if err != nil {
            return err // Transaction will be rolled back
        }
    }
    
    return nil // Transaction will be committed
})
```
