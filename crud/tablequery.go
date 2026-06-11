package crud

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type TqueryItem struct {
	Err   *string
	IsEnd bool
	Msg   proto.Message
}

func TableQueryBuildSql(db sqldb.DB, msgDesc protoreflect.MessageDescriptor, tableQueryReq *protodb.TableQueryReq, permissionSqlStr string, permissionSqlVals []any) (sqlStr string, sqlVals []interface{}, err error) {
	if err := validateTableQueryIdentifiers(msgDesc, tableQueryReq); err != nil {
		return "", nil, err
	}

	dbdialect := sqldb.GetExecutorDialect(db)
	placeholder := dbdialect.Placeholder()
	dbtableName := sqldb.BuildDbTableName(tableQueryReq.TableName, tableQueryReq.SchemeName, dbdialect)

	sb := strings.Builder{}
	sb.Grow(tableQueryBuildSQLCap(tableQueryReq, permissionSqlStr, len(permissionSqlVals), dbtableName, placeholder))
	sb.WriteString(protosql.SQL_SELECT)

	// Handle result columns
	if len(tableQueryReq.ResultColumnNames) == 0 {
		sb.WriteString(protosql.SQL_ASTERISK)
	} else {
		sb.WriteString(strings.Join(tableQueryReq.ResultColumnNames, protosql.SQL_COMMA))
	}

	sb.WriteString(protosql.SQL_FROM)

	// Build table name
	sb.WriteString(dbtableName)

	// Handle WHERE clauses
	sqlParaNo := 1

	firstPlaceholder := true

	if len(tableQueryReq.Where) > 0 || len(permissionSqlStr) > 0 {

		sb.WriteString(protosql.SQL_WHERE)

		// Add permission SQL if provided
		if len(permissionSqlStr) > 0 {
			sb.WriteString(protosql.SQL_LEFT_PARENTHESES)
			sb.WriteString(permissionSqlStr)
			sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
			firstPlaceholder = false
			if len(permissionSqlVals) > 0 {
				sqlVals = append(sqlVals, permissionSqlVals...)
				sqlParaNo += len(permissionSqlVals)
			}
		}

		// Add conditions from where map
		for fieldName, fieldValue := range tableQueryReq.Where {
			if !firstPlaceholder {
				sb.WriteString(protosql.SQL_AND)
			}
			firstPlaceholder = false

			sb.WriteString(fieldName)
			sb.WriteString(protosql.SQL_EQUEAL)

			if placeholder == protosql.SQL_QUESTION {
				sb.WriteString(string(protosql.SQL_QUESTION))
			} else {
				sb.WriteString(string(protosql.SQL_DOLLAR))
				sb.WriteString(strconv.Itoa(sqlParaNo))
				sqlParaNo++
			}

			sqlVals = append(sqlVals, fieldValue)
		}
	}

	//handle where2
	if len(tableQueryReq.Where2) > 0 {
		if len(tableQueryReq.Where2) != len(tableQueryReq.Where2Operator) {
			return "", nil, fmt.Errorf("where2 and where2Operator must have same length")
		}

		if firstPlaceholder {
			sb.WriteString(protosql.SQL_WHERE)
		}

		for fieldname, fieldValue := range tableQueryReq.Where2 {
			fieldWhereOperator, ok := tableQueryReq.Where2Operator[fieldname]
			if !ok {
				return "", nil, fmt.Errorf("where2 field %s has no operator provided", fieldname)
			}

			if firstPlaceholder {
				//has written where before
				firstPlaceholder = false
			} else {
				sb.WriteString(protosql.SQL_AND)
			}

			fieldDesc, err := getTableQueryFieldDesc(msgDesc, fieldname, "where2 field")
			if err != nil {
				return "", nil, err
			}

			condStr, condArgs, argInc, err := buildWhere2ConditionForColumn(dbdialect, placeholder, sqlParaNo, fieldname, fieldDesc, fieldWhereOperator, fieldValue)
			if err != nil {
				return "", nil, err
			}
			sb.WriteString(condStr)
			sqlVals = append(sqlVals, condArgs...)
			sqlParaNo += argInc
		}
	}

	// Add LIMIT and OFFSET if specified
	if tableQueryReq.Limit > 0 {
		sb.WriteString(protosql.SQL_LIMIT)
		sb.WriteString(strconv.FormatInt(int64(tableQueryReq.Limit), 10))
	}

	if tableQueryReq.Offset > 0 {
		sb.WriteString(protosql.SQL_OFFSET)
		sb.WriteString(strconv.FormatInt(tableQueryReq.Offset, 10))
	}

	sqlStr = sb.String()
	return sqlStr, sqlVals, nil
}

func tableQueryBuildSQLCap(tableQueryReq *protodb.TableQueryReq, permissionSqlStr string, permissionSqlValCount int, dbtableName string, placeholder protosql.SQLPlaceholder) int {
	capacity := len(protosql.SQL_SELECT)
	if len(tableQueryReq.ResultColumnNames) == 0 {
		capacity += len(protosql.SQL_ASTERISK)
	} else {
		for i, columnName := range tableQueryReq.ResultColumnNames {
			if i > 0 {
				capacity += len(protosql.SQL_COMMA)
			}
			capacity += len(columnName)
		}
	}

	capacity += len(protosql.SQL_FROM) + len(dbtableName)

	firstPlaceholder := true
	sqlParaNo := 1
	if len(tableQueryReq.Where) > 0 || len(permissionSqlStr) > 0 {
		capacity += len(protosql.SQL_WHERE)
		if len(permissionSqlStr) > 0 {
			capacity += len(protosql.SQL_LEFT_PARENTHESES) + len(permissionSqlStr) + len(protosql.SQL_RIGHT_PARENTHESES)
			firstPlaceholder = false
			sqlParaNo += permissionSqlValCount
		}
		for fieldName := range tableQueryReq.Where {
			if !firstPlaceholder {
				capacity += len(protosql.SQL_AND)
			}
			firstPlaceholder = false
			capacity += len(fieldName) + len(protosql.SQL_EQUEAL) + sqlPlaceholderCap(placeholder, sqlParaNo)
			if placeholder != protosql.SQL_QUESTION {
				sqlParaNo++
			}
		}
	}

	if len(tableQueryReq.Where2) > 0 {
		if firstPlaceholder {
			capacity += len(protosql.SQL_WHERE)
		}
		for fieldName := range tableQueryReq.Where2 {
			if firstPlaceholder {
				firstPlaceholder = false
			} else {
				capacity += len(protosql.SQL_AND)
			}
			capacity += tableQueryWhere2ConditionCap(fieldName, placeholder, sqlParaNo)
			if placeholder != protosql.SQL_QUESTION {
				sqlParaNo++
			}
		}
	}

	if tableQueryReq.Limit > 0 {
		capacity += len(protosql.SQL_LIMIT) + decimalDigitCount64(int64(tableQueryReq.Limit))
	}
	if tableQueryReq.Offset > 0 {
		capacity += len(protosql.SQL_OFFSET) + decimalDigitCount64(tableQueryReq.Offset)
	}
	return capacity
}

func tableQueryWhere2ConditionCap(fieldName string, placeholder protosql.SQLPlaceholder, paraNo int) int {
	return 192 + len(fieldName)*3 + sqlPlaceholderCap(placeholder, paraNo)*2
}

// validateTableQueryIdentifiers keeps RPC-supplied table/condition identifiers tied to the proto descriptor.
// DB schemas often use lowercase names while proto messages use exported Go-style names, so table and field
// checks are case-insensitive; ResultColumnNames are expression-capable and are validated separately.
func validateTableQueryIdentifiers(msgDesc protoreflect.MessageDescriptor, tableQueryReq *protodb.TableQueryReq) error {
	if tableQueryReq == nil {
		return fmt.Errorf("table query request is nil")
	}

	if len(tableQueryReq.SchemeName) > 0 {
		if err := validateTableQueryIdentifierSegment("schema", tableQueryReq.SchemeName); err != nil {
			return err
		}
	}

	expectedTableName := string(msgDesc.Name())
	if !strings.EqualFold(tableQueryReq.TableName, expectedTableName) {
		return fmt.Errorf("table name %s does not match message %s", tableQueryReq.TableName, expectedTableName)
	}
	if err := validateTableQueryIdentifierSegment("table", tableQueryReq.TableName); err != nil {
		return err
	}

	if err := validateTableQueryResultColumns(msgDesc, tableQueryReq.ResultColumnNames); err != nil {
		return err
	}

	for fieldName := range tableQueryReq.Where {
		if _, err := getTableQueryFieldDesc(msgDesc, fieldName, "where field"); err != nil {
			return err
		}
	}

	for fieldName := range tableQueryReq.Where2 {
		if _, err := getTableQueryFieldDesc(msgDesc, fieldName, "where2 field"); err != nil {
			return err
		}
	}

	for fieldName := range tableQueryReq.Where2Operator {
		if _, err := getTableQueryFieldDesc(msgDesc, fieldName, "where2 operator field"); err != nil {
			return err
		}
	}

	return nil
}

// validateTableQueryResultColumns allows projection expressions such as trim(col) or col::integer.
// This intentionally does not require a proto-field match because SQL result expressions may be aliased
// by callers; it only applies the expression-level injection guard used for SELECT list inputs.
func validateTableQueryResultColumns(msgDesc protoreflect.MessageDescriptor, resultColumns []string) error {
	if len(resultColumns) == 0 {
		return nil
	}
	if len(resultColumns) == 1 && strings.TrimSpace(resultColumns[0]) == "*" {
		return nil
	}

	for _, fieldName := range resultColumns {
		if strings.TrimSpace(fieldName) == "*" {
			return fmt.Errorf("result column * must be the only result column")
		}
		if err := checkSQLColumnsIsNoInjectionInWhere(fieldName); err != nil {
			return fmt.Errorf("check result column %s err: %w", fieldName, err)
		}
	}
	return nil
}

// getTableQueryFieldDesc resolves condition fields against the descriptor before they enter WHERE SQL.
// It accepts DB-style lowercase names as aliases for exported proto field names.
func getTableQueryFieldDesc(msgDesc protoreflect.MessageDescriptor, fieldName string, label string) (protoreflect.FieldDescriptor, error) {
	if err := validateTableQueryIdentifierSegment(label, fieldName); err != nil {
		return nil, err
	}
	fieldDesc := msgDesc.Fields().ByName(protoreflect.Name(fieldName))
	if fieldDesc == nil {
		fieldDesc = getTableQueryFieldDescFold(msgDesc, fieldName)
	}
	if fieldDesc == nil {
		return nil, fmt.Errorf("%s %s not found in message %s", label, fieldName, msgDesc.FullName())
	}
	return fieldDesc, nil
}

// getTableQueryFieldDescFold is the lowercase DB-name compatibility path for proto fields.
func getTableQueryFieldDescFold(msgDesc protoreflect.MessageDescriptor, fieldName string) protoreflect.FieldDescriptor {
	fields := msgDesc.Fields()
	for i := 0; i < fields.Len(); i++ {
		fieldDesc := fields.Get(i)
		if strings.EqualFold(fieldName, string(fieldDesc.Name())) || strings.EqualFold(fieldName, fieldDesc.TextName()) {
			return fieldDesc
		}
	}
	return nil
}

// validateTableQueryIdentifierSegment rejects qualified names here because schema/table/where fields are
// assembled by the builder; allowing dots would let callers cross table/schema boundaries.
func validateTableQueryIdentifierSegment(label string, name string) error {
	if strings.TrimSpace(name) == "*" {
		return fmt.Errorf("%s cannot be *", label)
	}
	if strings.Contains(name, ".") {
		return fmt.Errorf("%s %s must be an unqualified identifier", label, name)
	}
	if err := checkSQLColumnsIsNoInjectionStrict(name); err != nil {
		return fmt.Errorf("check %s %s err: %w", label, name, err)
	}
	return nil
}

// buildWhere2Condition preserves the historical helper behavior by using the descriptor text name as SQL column.
func buildWhere2Condition(dialect sqldb.TDBDialect, placeholder protosql.SQLPlaceholder, paraNo int, fieldDesc protoreflect.FieldDescriptor, op protodb.WhereOperator, valueStr string) (cond string, args []any, argInc int, err error) {
	return buildWhere2ConditionForColumn(dialect, placeholder, paraNo, fieldDesc.TextName(), fieldDesc, op, valueStr)
}

// buildWhere2ConditionForColumn uses fieldDesc for type semantics while keeping fieldName for emitted SQL.
// This lets lowercase DB column names pass validation without rewriting them back to exported proto names.
func buildWhere2ConditionForColumn(dialect sqldb.TDBDialect, placeholder protosql.SQLPlaceholder, paraNo int, fieldName string, fieldDesc protoreflect.FieldDescriptor, op protodb.WhereOperator, valueStr string) (cond string, args []any, argInc int, err error) {
	if fieldDesc.IsMap() {
		switch dialect {
		case sqldb.Postgres:
			switch op {
			case protodb.WhereOperator_WOP_HAS_KEY:
				cond = fieldName + " ? " + buildPlaceholder(placeholder, paraNo)
				return cond, []any{valueStr}, 1, nil
			case protodb.WhereOperator_WOP_CONTAINS:
				cond = fieldName + " @> " + buildPlaceholder(placeholder, paraNo) + "::jsonb"
				return cond, []any{valueStr}, 1, nil
			default:
				return "", nil, 0, fmt.Errorf("unsupported operator %v for map field %s", op, fieldName)
			}
		case sqldb.Mysql:
			switch op {
			case protodb.WhereOperator_WOP_HAS_KEY:
				// NOTE: assumes map keys can be addressed as $.<key>.
				cond = "JSON_CONTAINS_PATH(" + fieldName + ", 'one', CONCAT('$.' , " + buildPlaceholder(placeholder, paraNo) + "))"
				return cond, []any{valueStr}, 1, nil
			case protodb.WhereOperator_WOP_CONTAINS:
				cond = "JSON_CONTAINS(" + fieldName + ", CAST(" + buildPlaceholder(placeholder, paraNo) + " AS JSON))"
				return cond, []any{valueStr}, 1, nil
			default:
				return "", nil, 0, fmt.Errorf("unsupported operator %v for map field %s", op, fieldName)
			}
		case sqldb.SQLite:
			switch op {
			case protodb.WhereOperator_WOP_HAS_KEY:
				cond = "EXISTS (SELECT 1 FROM json_each(" + fieldName + ") WHERE key = " + buildPlaceholder(placeholder, paraNo) + ")"
				return cond, []any{valueStr}, 1, nil
			case protodb.WhereOperator_WOP_CONTAINS:
				// valueStr must be a JSON object string
				cond = "NOT EXISTS (SELECT 1 FROM json_each(" + buildPlaceholder(placeholder, paraNo) + ") b WHERE NOT EXISTS (SELECT 1 FROM json_each(" + fieldName + ") a WHERE a.key = b.key AND a.value = b.value))"
				return cond, []any{valueStr}, 1, nil
			default:
				return "", nil, 0, fmt.Errorf("unsupported operator %v for sqlite map field %s", op, fieldName)
			}
		default:
			return "", nil, 0, fmt.Errorf("unsupported dialect %v", dialect)
		}
	}
	if !fieldDesc.IsList() {
		// keep backward compatibility: treat value as string for scalar ops
		switch op {
		case protodb.WhereOperator_WOP_GT, protodb.WhereOperator_WOP_LT, protodb.WhereOperator_WOP_GTE, protodb.WhereOperator_WOP_LTE, protodb.WhereOperator_WOP_LIKE, protodb.WhereOperator_WOP_EQ:
			cond = fieldName + WhereOperator2Str(op) + buildPlaceholder(placeholder, paraNo)
			return cond, []any{valueStr}, 1, nil
		default:
			return "", nil, 0, fmt.Errorf("unsupported operator %v for non-list field %s", op, fieldName)
		}
	}

	// list operators
	switch dialect {
	case sqldb.Postgres:
		if fieldDesc.Kind() == protoreflect.MessageKind {
			switch op {
			case protodb.WhereOperator_WOP_CONTAINS, protodb.WhereOperator_WOP_CONTAINS_ALL:
				cond = fieldName + " @> " + buildPlaceholder(placeholder, paraNo) + "::jsonb"
				return cond, []any{valueStr}, 1, nil
			case protodb.WhereOperator_WOP_LEN_GT, protodb.WhereOperator_WOP_LEN_GTE, protodb.WhereOperator_WOP_LEN_LT, protodb.WhereOperator_WOP_LEN_LTE:
				cmp := lenOpToSql(op)
				cond = "jsonb_array_length(" + fieldName + ")" + cmp + buildPlaceholder(placeholder, paraNo)
				n, err := strconv.ParseInt(strings.TrimSpace(valueStr), 10, 64)
				if err != nil {
					return "", nil, 0, err
				}
				return cond, []any{n}, 1, nil
			default:
				return "", nil, 0, fmt.Errorf("unsupported list operator %v for repeated message field %s", op, fieldName)
			}
		}

		elemType := protodb.GetProtoDBType(fieldDesc.Kind(), sqldb.Postgres)
		switch op {
		case protodb.WhereOperator_WOP_CONTAINS:
			scalar, err := parseScalarString(fieldDesc.Kind(), valueStr)
			if err != nil {
				return "", nil, 0, err
			}
			cond = fieldName + " @> ARRAY[" + buildPlaceholder(placeholder, paraNo) + "]::" + elemType + "[]"
			return cond, []any{scalar}, 1, nil
		case protodb.WhereOperator_WOP_OVERLAP, protodb.WhereOperator_WOP_CONTAINS_ALL:
			arr, err := parseScalarJSONArray(fieldDesc.Kind(), valueStr)
			if err != nil {
				return "", nil, 0, err
			}
			opStr := " && "
			if op == protodb.WhereOperator_WOP_CONTAINS_ALL {
				opStr = " @> "
			}
			cond = fieldName + opStr + buildPlaceholder(placeholder, paraNo)
			return cond, []any{arr}, 1, nil
		case protodb.WhereOperator_WOP_LEN_GT, protodb.WhereOperator_WOP_LEN_GTE, protodb.WhereOperator_WOP_LEN_LT, protodb.WhereOperator_WOP_LEN_LTE:
			cmp := lenOpToSql(op)
			cond = "cardinality(" + fieldName + ")" + cmp + buildPlaceholder(placeholder, paraNo)
			n, err := strconv.ParseInt(strings.TrimSpace(valueStr), 10, 64)
			if err != nil {
				return "", nil, 0, err
			}
			return cond, []any{n}, 1, nil
		default:
			return "", nil, 0, fmt.Errorf("unsupported list operator %v for field %s", op, fieldName)
		}
	case sqldb.SQLite:
		switch op {
		case protodb.WhereOperator_WOP_CONTAINS:
			scalar, err := parseScalarString(fieldDesc.Kind(), valueStr)
			if err != nil {
				return "", nil, 0, err
			}
			cond = "EXISTS (SELECT 1 FROM json_each(" + fieldName + ") WHERE value = " + buildPlaceholder(placeholder, paraNo) + ")"
			return cond, []any{scalar}, 1, nil
		case protodb.WhereOperator_WOP_OVERLAP:
			cond = "EXISTS (SELECT 1 FROM json_each(" + fieldName + ") a JOIN json_each(" + buildPlaceholder(placeholder, paraNo) + ") b ON a.value = b.value)"
			return cond, []any{valueStr}, 1, nil
		case protodb.WhereOperator_WOP_CONTAINS_ALL:
			cond = "NOT EXISTS (SELECT 1 FROM json_each(" + buildPlaceholder(placeholder, paraNo) + ") b WHERE NOT EXISTS (SELECT 1 FROM json_each(" + fieldName + ") a WHERE a.value = b.value))"
			return cond, []any{valueStr}, 1, nil
		case protodb.WhereOperator_WOP_LEN_GT, protodb.WhereOperator_WOP_LEN_GTE, protodb.WhereOperator_WOP_LEN_LT, protodb.WhereOperator_WOP_LEN_LTE:
			cmp := lenOpToSql(op)
			cond = "json_array_length(" + fieldName + ")" + cmp + buildPlaceholder(placeholder, paraNo)
			n, err := strconv.ParseInt(strings.TrimSpace(valueStr), 10, 64)
			if err != nil {
				return "", nil, 0, err
			}
			return cond, []any{n}, 1, nil
		default:
			return "", nil, 0, fmt.Errorf("unsupported list operator %v for sqlite field %s", op, fieldName)
		}
	case sqldb.Mysql:
		switch op {
		case protodb.WhereOperator_WOP_CONTAINS:
			if fieldDesc.Kind() == protoreflect.MessageKind {
				cond = "JSON_CONTAINS(" + fieldName + ", JSON_ARRAY(CAST(" + buildPlaceholder(placeholder, paraNo) + " AS JSON)))"
				return cond, []any{valueStr}, 1, nil
			}
			scalar, err := parseScalarString(fieldDesc.Kind(), valueStr)
			if err != nil {
				return "", nil, 0, err
			}
			cond = "JSON_CONTAINS(" + fieldName + ", JSON_ARRAY(" + buildPlaceholder(placeholder, paraNo) + "))"
			return cond, []any{scalar}, 1, nil
		case protodb.WhereOperator_WOP_OVERLAP:
			cond = "JSON_OVERLAPS(" + fieldName + ", CAST(" + buildPlaceholder(placeholder, paraNo) + " AS JSON))"
			return cond, []any{valueStr}, 1, nil
		case protodb.WhereOperator_WOP_CONTAINS_ALL:
			cond = "JSON_CONTAINS(" + fieldName + ", CAST(" + buildPlaceholder(placeholder, paraNo) + " AS JSON))"
			return cond, []any{valueStr}, 1, nil
		case protodb.WhereOperator_WOP_LEN_GT, protodb.WhereOperator_WOP_LEN_GTE, protodb.WhereOperator_WOP_LEN_LT, protodb.WhereOperator_WOP_LEN_LTE:
			cmp := lenOpToSql(op)
			cond = "JSON_LENGTH(" + fieldName + ")" + cmp + buildPlaceholder(placeholder, paraNo)
			n, err := strconv.ParseInt(strings.TrimSpace(valueStr), 10, 64)
			if err != nil {
				return "", nil, 0, err
			}
			return cond, []any{n}, 1, nil
		default:
			return "", nil, 0, fmt.Errorf("unsupported list operator %v for mysql field %s", op, fieldName)
		}
	default:
		return "", nil, 0, fmt.Errorf("unsupported dialect %v", dialect)
	}
}

func buildPlaceholder(placeholder protosql.SQLPlaceholder, paraNo int) string {
	if placeholder == protosql.SQL_QUESTION {
		return string(protosql.SQL_QUESTION)
	}
	return string(protosql.SQL_DOLLAR) + strconv.Itoa(paraNo)
}

func lenOpToSql(op protodb.WhereOperator) string {
	switch op {
	case protodb.WhereOperator_WOP_LEN_GT:
		return protosql.SQL_GT
	case protodb.WhereOperator_WOP_LEN_GTE:
		return protosql.SQL_GTE
	case protodb.WhereOperator_WOP_LEN_LT:
		return protosql.SQL_LT
	case protodb.WhereOperator_WOP_LEN_LTE:
		return protosql.SQL_LTE
	default:
		return ""
	}
}

func parseScalarString(kind protoreflect.Kind, s string) (any, error) {
	ss := strings.TrimSpace(s)
	switch kind {
	case protoreflect.BoolKind:
		if ss == "1" {
			return true, nil
		}
		if ss == "0" {
			return false, nil
		}
		b, err := strconv.ParseBool(strings.ToLower(ss))
		return b, err
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.EnumKind:
		i, err := strconv.ParseInt(ss, 10, 64)
		if err != nil {
			return nil, err
		}
		return i, nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		u, err := strconv.ParseUint(ss, 10, 64)
		if err != nil {
			return nil, err
		}
		return int64(u), nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		f, err := strconv.ParseFloat(ss, 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	case protoreflect.StringKind:
		return s, nil
	default:
		return s, nil
	}
}

func parseScalarJSONArray(kind protoreflect.Kind, s string) (any, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, err
	}
	switch kind {
	case protoreflect.StringKind:
		out := make([]string, 0, len(raw))
		for _, r := range raw {
			var v string
			if err := json.Unmarshal(r, &v); err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case protoreflect.BoolKind:
		out := make([]bool, 0, len(raw))
		for _, r := range raw {
			var v bool
			if err := json.Unmarshal(r, &v); err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		out := make([]float64, 0, len(raw))
		for _, r := range raw {
			var v float64
			if err := json.Unmarshal(r, &v); err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	default:
		out := make([]int64, 0, len(raw))
		for _, r := range raw {
			var v any
			if err := json.Unmarshal(r, &v); err != nil {
				return nil, err
			}
			iv, err := parseScalarString(kind, fmt.Sprint(v))
			if err != nil {
				return nil, err
			}
			out = append(out, iv.(int64))
		}
		return out, nil
	}
}

func WhereOperator2Str(fieldop protodb.WhereOperator) string {
	switch fieldop {
	case protodb.WhereOperator_WOP_GT:
		return protosql.SQL_GT
	case protodb.WhereOperator_WOP_LT:
		return protosql.SQL_LT
	case protodb.WhereOperator_WOP_GTE:
		return protosql.SQL_GTE
	case protodb.WhereOperator_WOP_LTE:
		return protosql.SQL_LTE
	case protodb.WhereOperator_WOP_LIKE:
		return protosql.SQL_LIKE
	case protodb.WhereOperator_WOP_EQ:
		return protosql.SQL_EQUEAL
	default:
		return " unsupported operator: " + fmt.Sprint(fieldop)
	}
}
