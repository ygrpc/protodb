# feat: repeated field as array

## Background

Current `protodb` maps protobuf message fields to SQL columns primarily by `protoreflect.FieldDescriptor.Kind()`.

- DDL uses `GetProtoDBType*` and does **not** check `fieldDesc.IsList()`.
- Insert/Update only special-cases `MessageKind` with `protojson.Marshal`, but does not encode repeated lists.
- Scan/Set field logic ultimately relies on `pdbutil.SetField` which does not support slice/list assignment.
- TableQuery where builder is string-based and does not know the protobuf field descriptor/type, so it cannot implement array semantics.

Goal: add first-class support for `repeated` fields stored as **DB arrays** (PostgreSQL) and **JSON arrays** (SQLite), including reasonable `WHERE` support.

## Scope / Requirements

- Primary dialect: **PostgreSQL**.
- Compatibility: **SQLite** (MySQL can be ignored for now).
- Supported repeated types:
  - **Required**: repeated scalar + enum.
  - **Nice-to-have**: repeated message.
- Query: **as much as possible** for `WHERE`.
- Storage semantics:
  - DB should store **empty array** by default (not NULL).
  - When scanning back, NULL and empty are both acceptable in protobuf; recommended to normalize to **empty list**.

## High-level design

Introduce a dialect-aware "List storage strategy":

- PostgreSQL:
  - repeated scalar/enum -> **native SQL array** columns (`<elem_type>[]`).
  - repeated message -> **jsonb** (JSON array of objects).
- SQLite:
  - all repeated types -> **JSON array** stored as `TEXT` (`'[]'` default).

Additionally:

- Force repeated columns to be `NOT NULL` and with default empty array (`DEFAULT '{}'::type[]` for PG arrays, `DEFAULT '[]'` for JSON).
- Add array-aware encoding/decoding for insert/update and scan.
- Extend `WhereOperator` to support list semantics.
- Make the TableQuery builder type-aware by passing `protoreflect.MessageDescriptor` (or a message instance) down.

## Storage model

### PostgreSQL mapping

#### repeated scalar/enum -> native arrays

Recommended mapping (consistent with existing scalar mapping in `pdbfield.go`):

- `repeated bool` -> `boolean[]`
- `repeated int32/sint32/sfixed32` -> `integer[]`
- `repeated int64/sint64/sfixed64` -> `bigint[]`
- `repeated uint32/fixed32` -> `bigint[]` (keeps existing "uint32 -> bigint" approach)
- `repeated uint64/fixed64` -> `bigint[]` (same)
- `repeated float` -> `real[]`
- `repeated double` -> `double precision[]`
- `repeated string` -> `text[]`
- `repeated bytes` -> `bytea[]`
- `repeated enum` -> `integer[]`

#### repeated message -> jsonb

- `repeated SomeMessage` -> `jsonb`
- Stored value: JSON array of message objects.

Rationale:

- PG "array of composite types" requires defining composite types / casts, which is heavy.
- `jsonb` provides practical querying (`@>`) and indexing (GIN).

### SQLite mapping

SQLite lacks a true array type.

- repeated scalar/enum/message -> `TEXT NOT NULL DEFAULT '[]'`
- Contents are JSON arrays.

Dependency:

- SQLite JSON1 extension for efficient queries.

## DDL changes

### Type resolution

Current DDL uses `getSqlTypeStr(fieldDesc, fieldPdb, dialect)` which delegates to `fieldPdb.PdbDbTypeStr(...)/GetProtoDBType*`.

Add list-aware type resolution:

1) If `fieldDesc.IsList()`:

- If user sets `DbTypeStr` or `DbType`, use user type first.
- Otherwise use generated type by dialect/kind.

- Postgres:
  - elem kind is scalar/enum -> `<elemSqlType>[]`
  - elem kind is message -> `jsonb`
- SQLite:
  - always `text`

2) Else: keep current behavior.

### Default value / NOT NULL policy

Decision: repeated fields are **forced** to `NOT NULL` and have default empty list.

- Postgres native arrays:
  - `NOT NULL DEFAULT '{}'::<elemType>[]`
  - Example: `tags text[] NOT NULL DEFAULT '{}'::text[]`
- Postgres jsonb (repeated message):
  - `NOT NULL DEFAULT '[]'::jsonb`
- SQLite JSON text:
  - `NOT NULL DEFAULT '[]'`

Note:

- This is independent of `pdb.NotNull` / `pdb.DefaultValue` because the requirement is global.
- If you want an escape hatch later, extend `PDBField` with e.g. `DisableDefaultEmptyList`.

### Indexing recommendation

- Postgres native arrays:
  - For containment/overlap queries: recommend `GIN` index on array column.
    - `CREATE INDEX ... ON t USING gin (col);`
- Postgres jsonb:
  - `GIN` index:
    - `CREATE INDEX ... ON t USING gin (col jsonb_path_ops);` (or default gin)
- SQLite:
  - JSON1 queries are harder to index. Usually acceptable for small-medium data; otherwise consider an auxiliary index table (see Alternatives).

## CRUD encoding (insert/update)

Introduce a single encoding function used by insert/update:

- `EncodeSQLArg(fieldDesc, dialect, goValue) (interface{}, error)`

Rules:

### repeated scalar/enum

- Normalize nil slice -> empty slice.
- Postgres:
  - Bind as SQL array.
  - Implementation notes:
    - If using `pgx stdlib` driver: many basic slices are supported.
    - If using `lib/pq`: need `pq.Array(slice)`.
  - Recommended abstraction:
    - Add `sqldb.ArrayArg(dialect, slice)` that returns the correct driver value.
- SQLite:
  - Marshal slice to JSON string via `encoding/json.Marshal`.

### repeated message

- Convert list to JSON array string:
  - Either marshal `[]any` produced by per-element `protojson.Marshal`.
  - Or use `protojson.Marshal` on a helper wrapper message; but simplest is manual.
- Store:
  - Postgres: bind JSON string/bytes to jsonb.
  - SQLite: bind JSON string.

### singular message (existing behavior)

Keep existing behavior:

- `MessageKind` (non-repeated) -> `protojson.Marshal(msg)` into string/bytes.

## Scan decoding (select)

Update `crud/dbscan.go:SetProtoMsgField` to handle lists.

### General approach

Avoid `pdbutil.SetField` for repeated; use `protoreflect` APIs:

- `m := msg.ProtoReflect()`
- `list := m.Mutable(fieldDesc).List()`
- Clear and append decoded elements.

Normalize NULL / empty:

- If DB value is NULL -> treat as empty list.

### PostgreSQL arrays

Possible scan representations depend on driver:

- It may already be a Go slice (`[]int64`, `[]string`, ...).
- It may be textual (`"{1,2}"`) or `[]byte`.

Recommended decoding priority:

1) If value is a slice of element type -> append directly.
2) Else if value is `string` or `[]byte`:
   - Option A (simpler, robust): in SELECT, cast array to JSON using `array_to_json(col)`.
     - Pros: unified JSON decode path.
     - Cons: requires changing result SQL (and affects `SELECT *` semantics).
   - Option B (less invasive to SQL): implement minimal PG array text parser.

Given current `TableQuery` often does `SELECT *`, Option B is more compatible, but is more code; let's use Option B for better SQL compatibility.

### SQLite JSON

- Value expected as `string` or `[]byte` JSON.
- `json.Unmarshal` into `[]T` for scalar lists.
- For repeated message:
  - Unmarshal into `[]json.RawMessage` and `protojson.Unmarshal` each item.

## Query (WHERE) support

### Problem in current implementation

`crud/tablequery.go:TableQueryBuildSql` builds SQL purely from `map[string]string` and operators, without knowing:

- whether a field is repeated
- element type
- how to encode values

Therefore it cannot implement array contains/overlap semantics.

### Recommended change: make the builder type-aware

- Update `TableQueryBuildSql` signature to accept `msgDesc protoreflect.MessageDescriptor` (or a `proto.Message`).
- `service.HandleTableQuery` already loads `dbmsg := msgstore.GetMsg(...)`; pass its descriptor.
- Validate field existence using descriptor rather than only injection checks.

### Extend WhereOperator

Add list-related operators to `protodb.proto`:

- `WOP_CONTAINS`        // array contains element
- `WOP_OVERLAP`         // array overlaps given array
- `WOP_CONTAINS_ALL`    // array contains all elements of given array
- `WOP_LEN_GT`          // length greater than
- `WOP_LEN_GTE`
- `WOP_LEN_LT`
- `WOP_LEN_LTE`

Keep existing operators for scalar fields.

### Value format for list operators

Because `Where2` is `map<string,string>`, we need a string encoding convention.

Recommended conventions:

- For `WOP_CONTAINS`:
  - value is a single scalar encoded as string (e.g. `"42"`, `"abc"`).
- For `WOP_OVERLAP` / `WOP_CONTAINS_ALL`:
  - value is a JSON array string (e.g. `"[1,2,3]"`, `"[\"a\",\"b\"]"`).
- For repeated message contains:
  - value is JSON array/object pattern string for jsonb/json. (PG jsonb uses `@>` semantics.)

The builder should parse the string into typed parameters according to element kind.

### SQL generation per dialect

#### PostgreSQL native arrays (repeated scalar/enum)

- `WOP_CONTAINS` (element):
  - `col @> ARRAY[$n]::<elemType>[]`
- `WOP_OVERLAP` (list):
  - `col && $n` where `$n` is an array arg
- `WOP_CONTAINS_ALL` (list):
  - `col @> $n` where `$n` is an array arg
- `WOP_LEN_*`:
  - `cardinality(col) > $n` etc.

#### PostgreSQL jsonb (repeated message)

- `WOP_CONTAINS` / `WOP_CONTAINS_ALL`:
  - `col @> $n::jsonb` where `$n` is JSON array string
- length:
  - `jsonb_array_length(col) > $n`

#### SQLite JSON (all repeated)

- `WOP_CONTAINS` (scalar element):
  - `EXISTS (SELECT 1 FROM json_each(col) WHERE value = ?)`
- `WOP_OVERLAP` (list JSON array):
  - `EXISTS (SELECT 1 FROM json_each(col) a JOIN json_each(?) b ON a.value = b.value)`
- `WOP_CONTAINS_ALL`:
  - `NOT EXISTS (SELECT 1 FROM json_each(?) b WHERE NOT EXISTS (SELECT 1 FROM json_each(col) a WHERE a.value = b.value))`
- length:
  - `json_array_length(col) > ?`

Note:

- For SQLite, passing `?` as JSON string for `json_each(?)` is supported.

## Code touch points (implementation map)

### DDL

- `ddl/create.go`:
  - where columns are emitted, detect `fieldDesc.IsList()` and use list-aware type + default.
- `protodb/pdbfield.go`:
  - optional: centralize list-aware type mapping functions to avoid scattering dialect checks.

### Insert/Update

- `crud/insert.go` / `crud/update.go`:
  - after `GetField`, before `vals=append(vals, val)`:
    - call `EncodeSQLArg(fieldDesc, dialect, val)`

### Scan

- `crud/dbscan.go:SetProtoMsgField`:
  - handle `fieldDesc.IsList()`:
    - decode based on dialect + element kind
    - set via `protoreflect.List`

### Query

- `crud/tablequery.go:TableQueryBuildSql`:
  - add descriptor arg
  - extend operator mapping
  - parse/encode values properly
- `protodb.proto`:
  - extend `WhereOperator` enum
  - regenerate pb/go/connect stubs as needed

## Alternatives / future enhancements

### Alternative A: normalize everything to JSON even on PostgreSQL

- Store repeated scalar/enum as `jsonb` too.
- Pros:
  - simpler encoding/decoding across dialects.
- Cons:
  - less type safety, potentially slower, and different indexing semantics.

### Alternative B: join table for repeated (normalized relational design)

- For field `repeated T f = N;` create table `Parent_f(ParentPK..., idx, value)`.
- Pros:
  - best query performance and indexing across all DBs.
- Cons:
  - heavy DDL changes, join logic, cascade operations.

Given the project’s current architecture (single table per message, no join planner), this is not recommended as the first step.

## Open decisions (already confirmed)

- Repeated fields are **default forced** `NOT NULL` and default empty array.
- `WhereOperator` enum **can be extended**.
