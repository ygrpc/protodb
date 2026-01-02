# feat: 支持 Map 字段

## 背景

目前的 `protodb` 主要通过 `protoreflect.FieldDescriptor.Kind()` 将 protobuf 消息字段映射到 SQL 列。

对于 `map` 字段 (例如 `map<string, int32>`):

- `proto.FieldDescriptor.IsMap()` 为 `true`。
- `proto.FieldDescriptor.Kind()` 返回的是 **Value Kind** (例如 `Int32Kind`)。
- `proto.FieldDescriptor.IsList()` 为 `false`。

当前的 DDL 逻辑 (`pdbfield.go`) 依赖于 `Kind()`。这导致 `map<string, int32>` 会被错误地映射为数据库中的 `integer` (或其他标量类型)。
当前的 CRUD 逻辑没有针对 Map 的特殊处理，因此在插入/扫描时可能会失败或产生错误行为。

## 目标

为 `map<K, V>` 字段添加原生支持，在所有支持的方言中将其存储为 **JSON 对象**。

## 范围 / 需求

- **支持的方言**: PostgreSQL, SQLite, MySQL。
- **前置条件**:
  - **PostgreSQL**: 支持 `jsonb`。
  - **MySQL**: 需支持 JSON 列的默认值表达式 (例如 `DEFAULT (CAST('{}' AS JSON))`)。
  - **SQLite**: 需启用 **JSON1** 扩展。
- **支持的 Map 类型**: 所有标准的 protobuf map 类型 (Key 为 integer/string/bool, Value 为任何标量或消息)。
- **存储格式**: JSON 对象。
- **默认值**: 空 JSON 对象 (`{}`)。

## 高层设计

处理 `map` 字段的方式类似于 "feat-array" 设计中处理 `repeated` 消息的方式，但在所有方言中严格使用 JSON 存储 (因为标准 SQL 数组无法很好地模拟 Map/字典)。

### 存储策略

| 方言 | DB 类型 | 默认值 | 备注 |
| :--- | :--- | :--- | :--- |
| **PostgreSQL** | `jsonb` | `'{}'::jsonb` | 支持高效索引和查询 (`@>`, `?`, `->`)。 |
| **MySQL** | `json` | `(CAST('{}' AS JSON))` | 若版本不支持函数默认值，需设为 NULL 或升级。 |
| **SQLite** | `text` | `'{}'` | 应用逻辑处理 JSON 编解码。建议加 `CHECK(json_valid(col))` 约束。 |

## DDL 变更

### 类型解析

修改 DDL 生成逻辑 (例如在 `pdbfield.go` 的 `PdbDbTypeStr` 或调用者中)，确立以下检查顺序：

1. **首先检查 `fieldDesc.IsMap()`**:
   - 如果为 `true`:
     - **PostgreSQL**: 返回 `jsonb`。
     - **MySQL**: 返回 `json`。
     - **SQLite**: 返回 `text` (或 `text` 别名)。
2. **其次检查 `fieldDesc.IsList()`** (feat-array 逻辑):
   - 如果为 `true`，按数组逻辑处理。
3. **最后检查 `Kind()`**:
   - 如果上述都不是，按现有标量逻辑处理。

### 默认值

- 强制 `NOT NULL`。
- 默认值必须是空 JSON 对象 `{}`。
  - PG: `DEFAULT '{}'::jsonb`
  - SQLite: `DEFAULT '{}'`
  - MySQL: `DEFAULT (CAST('{}' AS JSON))`

## CRUD 变更 (运行时)

### 编码 (Insert/Update)

在 `crud/insert.go` / `crud/update.go` (特别是 feat-array 引入的 `EncodeSQLArg` 或等效逻辑) 中:

- 如果 `fieldDesc.IsMap()`:
  - 编组 (Marshaling): 统一编码为 **JSON object**。
  - **非从属类型的 Key 处理**:
    - Protobuf map 键可以是整数 (`int32`, `int64` 等) 或布尔值。
    - JSON 对象的键**必须**是字符串。
    - **策略**: 遵循 `protojson` 规范。在编码时，将整数/布尔键转换为字符串 (例: `123` -> `"123"`, `true` -> `"true"`)。
  - **实现建议**:
    - 手动迭代 `protoreflect.Map`，将 Key 转为 `string`，构建 `map[string]json.RawMessage`。
      - Value 为 **Message**: 用 `protojson.Marshal`。
      - Value 为 **标量**: 用 `encoding/json.Marshal` (避免 `interface{}` 带来的 `float64` 精度/类型问题)。
    - 最后将上述 map 编为 JSON 文本作为 SQL 参数。

### 解码 (Scan)

在 `crud/dbscan.go` 中:

- 如果 `fieldDesc.IsMap()`:
  - 数据库返回 `[]byte` 或 `string` (JSON)。
  - 解组 (Unmarshal):
    - **非从属类型的 Key 处理**:
      - JSON 解析后，键是字符串。必须将其解析回 Protobuf 定义的 Key 类型 (`int32`, `bool` 等)。
      - 例如: JSON `{"123": "val"}` -> Proto Map `123: "val"` (KeyType=INT32)。
      - 需要根据 `fieldDesc.MapKey().Kind()` 进行相应的 `strconv.ParseInt` / `ParseBool` 等操作。
    - **数据流**:
      1. `json.Unmarshal` 到 `map[string]json.RawMessage` (延迟解析 Value)。
      2. 遍历 map。
      3. 将 String Key 解析为 Proto Key。
      4. 将 Value 部分根据 `MapValue` 的类型进行解析 (若是 Message 则递归 `protojson`，若是标量则类型转换)。
      5. 填充到 `msg.Mutable(field).Map()`。

## 查询支持 (WHERE)

我们需要扩展 `TableQuery` 以支持 map 操作。

### 新增 / 扩展的操作符

复用或在 `protodb.proto` 中添加 `WhereOperator` 值:

1. **`WOP_HAS_KEY`** (新增): 检查 map 是否包含特定键。
    - **值**: 字符串形式的键 (例如 "my_key", "123")。
    - **SQL 生成**:
        - **PG**: `col ? $key`
        - **SQLite**: `EXISTS (SELECT 1 FROM json_each(col) WHERE key = $key)`
        - **MySQL**: `JSON_CONTAINS_PATH(col, 'one', CONCAT('$."', $key, '"'))`

2. **`WOP_CONTAINS`** (现有 - 复用): 检查 map 是否包含键值对子集 (Subset Match)。
    - **语义**: 当用于 Map 字段时，表示 JSON 对象包含语义 (`@>`)。即：左值 Map 包含右值 Map 的所有键，且对应的值相等。
    - **值**: JSON 对象字符串。例如 `{"color": "red", "size": 10}`。调用方应保证该 JSON 为 object。
    - **SQL 生成**:
        - **PG**: `col @> $value::jsonb`
        - **MySQL**: `JSON_CONTAINS(col, CAST($value AS JSON))`
        - **SQLite**: 基于 `json_each` 做包含校验 (单层键值包含):
            - `NOT EXISTS (
                SELECT 1
                FROM json_each($value) AS kv
                WHERE json_extract(col, '$.' || kv.key) IS NULL
                   OR json_extract(col, '$.' || kv.key) != kv.value
              )`
            - 注: 以上要求 `$value` 为 object，且对深层结构/数组的等价性判定可在后续增强。

### 索引建议

为了确保查询性能，特别是针对 Map 的查询，建议如下：

- **PostgreSQL**:
  - 对于所有的 Map (jsonb) 字段，强烈建议创建 **GIN 索引**。
  - SQL: `CREATE INDEX ON table_name USING gin (column_name);`
  - 这支持高效的 `@>` (WOP_CONTAINS), `?` (WOP_HAS_KEY), `?&`, `?|` 操作。

- **MySQL / SQLite**:
  - 对 JSON 内部字段建立索引通常需要创建 "生成列" (Generated Columns) 并对该列建立索引。对于动态 Key 的 Map，难以预先建立特定 Key 的索引。

## 实施计划

1. **Proto 定义**:
    - 向 `WhereOperator` 添加 `WOP_HAS_KEY`。
2. **DDL**:
    - 更新 `pdbfield.go` 逻辑，确保先判断 `IsMap()`。
    - 更新 `CreateTable` SQL 生成，处理 JSON 类型及默认值。
3. **CRUD**:
    - 实现 Map 的通用编解码逻辑 (处理 Key 类型转换)。
4. **查询**:
    - 实现 Map 操作符的 SQL 生成适配器。

## 测试用例 / 验收标准

### DDL

- **列类型正确**:
  - PostgreSQL: `map<K,V>` 列类型为 `jsonb`。
  - MySQL: `map<K,V>` 列类型为 `json`。
  - SQLite: `map<K,V>` 列类型为 `text`。
- **默认值与非空**:
  - 三方言均为 `NOT NULL`，且默认值为 `{}`。
  - 插入时不显式指定该列，应得到 `{}`。
- **SQLite JSON 有效性 (若采用 CHECK 约束)**:
  - 向该列写入非 JSON 文本应失败。

### CRUD (Insert/Update/Scan)

准备一个包含多种 map 字段的消息（示例语义）：

- `map<string, int32>`
- `map<int32, string>`
- `map<bool, string>`
- `map<string, SubMsg>` (value 为 message)

验收点:

- **空值/默认值**:
  - map 字段未设置或为空时，落库值应为 `{}` (而不是 `NULL`)。
- **Key 编码**:
  - `map<int32, string>{1:"a"}` 落库 JSON 应为 `{"1":"a"}`。
  - `map<bool, string>{true:"t"}` 落库 JSON 应为 `{"true":"t"}`。
- **Key 解码**:
  - 从 JSON `{"1":"a"}` 扫描回 `map<int32,string>` 时，key 应为整数 `1`。
  - 从 JSON `{"true":"t"}` 扫描回 `map<bool,string>` 时，key 应为布尔 `true`。
  - 对非法 key（例如 `{"x":"a"}` 扫描到 `map<int32,string>`）应报错。
- **Value 编码/解码 (标量)**:
  - `map<string,int32>{"n":123}` 往返后仍为 123。
- **Value 编码/解码 (message)**:
  - `map<string,SubMsg>{"k":{...}}` 往返后 message 内容等价 (字段值一致)。

### WHERE (查询)

准备测试数据:

- RowA: `m = {"color":"red","size":10}`
- RowB: `m = {"color":"blue"}`
- RowC: `m = {}`

验收点:

- **WOP_HAS_KEY**:
  - 查询 key=`"color"` 应命中 RowA/RowB，不命中 RowC。
  - 查询 key=`"size"` 应仅命中 RowA。
- **WOP_CONTAINS (子集包含)**:
  - value=`{"color":"red"}` 应命中 RowA。
  - value=`{"color":"red","size":10}` 应命中 RowA。
  - value=`{"size":10}` 应命中 RowA。
  - value=`{"color":"red","size":11}` 不应命中任何行。
  - value=`{}` 应命中所有行。

方言一致性要求:

- PostgreSQL / MySQL / SQLite 对上述测试用例结果保持一致（SQLite 允许性能较差但语义需一致）。

## 迁移 / 兼容性说明

- 若历史版本已经将 `map<K,V>` 字段当作标量类型创建过列 (例如误建为 `integer`/`text`)，升级到本特性后需要进行 schema 迁移。
- 建议做法:
  - 新建列为 JSON 类型/文本并回填为 `{}` 或从旧列转换；或
  - 重建表并重新导入数据。

## 备选方案 / 未来增强

- **未来的查询增强**:
  - **通配符查询**: 支持检查是否存在满足特定条件的值，忽略键 (例如: 所有的值都 > 10)。
  - **键集合获取**: 获取 Map 的所有 Key 列表。
- **Hstore (Postgres)**:
  - 仅支持 `string->string`。受限。`jsonb` 更优。
- **独立表** (`Map_TableName_Field`):
  - `(ParentID, Key, Value)` 模式。
  - 优点: 纯 SQL，外键，规范化。
  - 缺点: 基本 CRUD 复杂 (需要 Join/多次 Insert)，破坏了 "一个消息 = 一个表" 的简单性。
  - **结论**: 暂不采纳。为了简单和性能，坚持使用文档/JSON 方法。
