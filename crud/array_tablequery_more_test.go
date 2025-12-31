package crud

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestParseScalarString_AllKinds(t *testing.T) {
	cases := []struct {
		name string
		k    protoreflect.Kind
		in   string
		want any
	}{
		{"bool_1", protoreflect.BoolKind, "1", true},
		{"bool_0", protoreflect.BoolKind, "0", false},
		{"bool_true", protoreflect.BoolKind, "true", true},
		{"int64", protoreflect.Int64Kind, "-2", int64(-2)},
		{"uint64", protoreflect.Uint64Kind, "2", int64(2)},
		{"double", protoreflect.DoubleKind, "1.5", float64(1.5)},
		{"string", protoreflect.StringKind, " abc ", " abc "},
		{"enum", protoreflect.EnumKind, "3", int64(3)},
		{"bytes_default", protoreflect.BytesKind, "abc", "abc"},
		{"unknown_default", protoreflect.GroupKind, "xyz", "xyz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseScalarString(tc.k, tc.in)
			if err != nil {
				t.Fatalf("parseScalarString: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %#v, got %#v", tc.want, got)
			}
		})
	}
}

func TestBuildWhere2Condition_RepeatedMessage_PostgresLenLTE_AndParseError(t *testing.T) {
	_, _, _, subsField, _ := buildTestArrayDescriptors(t)

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 9, subsField, protodb.WhereOperator_WOP_LEN_LTE, "2")
	if err != nil {
		t.Fatalf("buildWhere2Condition len_lte: %v", err)
	}
	if cond != "jsonb_array_length(subs) <= $9" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != int64(2) {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}

	if _, _, _, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, subsField, protodb.WhereOperator_WOP_LEN_GT, "x"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWhere2Condition_RepeatedScalar_PostgresOverlap_InvalidJSON(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("ResultColumnNames")

	if _, _, _, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, field, protodb.WhereOperator_WOP_OVERLAP, "not-json"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWhere2Condition_RepeatedBool_PostgresContains_ParseError(t *testing.T) {
	fd := buildTestFileDescriptorProtoForScalarList(t, "bools", descriptorTypeBool)
	msgDesc := fd.Messages().ByName("TestMsg")
	field := msgDesc.Fields().ByName("bools")

	if _, _, _, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, field, protodb.WhereOperator_WOP_CONTAINS, "notabool"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseScalarJSONArray_AllKinds(t *testing.T) {
	// string
	got, err := parseScalarJSONArray(protoreflect.StringKind, "[\"a\",\"b\"]")
	if err != nil {
		t.Fatalf("parseScalarJSONArray string: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("unexpected string arr: %#v", got)
	}

	// bool
	got, err = parseScalarJSONArray(protoreflect.BoolKind, "[true,false]")
	if err != nil {
		t.Fatalf("parseScalarJSONArray bool: %v", err)
	}
	if !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatalf("unexpected bool arr: %#v", got)
	}

	// float
	got, err = parseScalarJSONArray(protoreflect.DoubleKind, "[1.25,2.5]")
	if err != nil {
		t.Fatalf("parseScalarJSONArray float: %v", err)
	}
	if !reflect.DeepEqual(got, []float64{1.25, 2.5}) {
		t.Fatalf("unexpected float arr: %#v", got)
	}

	// int fallback (also covers uint kind via parseScalarString)
	got, err = parseScalarJSONArray(protoreflect.Uint64Kind, "[1,2]")
	if err != nil {
		t.Fatalf("parseScalarJSONArray int: %v", err)
	}
	if !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Fatalf("unexpected int arr: %#v", got)
	}

	// enum falls into default int64 path
	got, err = parseScalarJSONArray(protoreflect.EnumKind, "[1,2]")
	if err != nil {
		t.Fatalf("parseScalarJSONArray enum: %v", err)
	}
	if !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Fatalf("unexpected enum arr: %#v", got)
	}
}

func TestParseScalarJSONArray_ErrorOnElemTypeMismatch(t *testing.T) {
	if _, err := parseScalarJSONArray(protoreflect.BoolKind, "[\"a\"]"); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := parseScalarJSONArray(protoreflect.DoubleKind, "[\"a\"]"); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := parseScalarJSONArray(protoreflect.Int64Kind, "[\"a\"]"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWhere2Condition_RepeatedMessage_Postgres(t *testing.T) {
	_, _, _, subsField, _ := buildTestArrayDescriptors(t)

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, subsField, protodb.WhereOperator_WOP_CONTAINS, "[{\"name\":\"a\"}]")
	if err != nil {
		t.Fatalf("buildWhere2Condition contains: %v", err)
	}
	if cond != "subs @> $1::jsonb" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 {
		t.Fatalf("unexpected args/inc: %v %d", args, inc)
	}

	cond, args, inc, err = buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 2, subsField, protodb.WhereOperator_WOP_LEN_GT, "3")
	if err != nil {
		t.Fatalf("buildWhere2Condition len: %v", err)
	}
	if cond != "jsonb_array_length(subs) > $2" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != int64(3) {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_RepeatedScalar_SQLiteContainsAllAndLen(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("ResultColumnNames")

	cond, args, inc, err := buildWhere2Condition(sqldb.SQLite, protosql.SQL_QUESTION, 1, field, protodb.WhereOperator_WOP_CONTAINS_ALL, "[\"a\",\"b\"]")
	if err != nil {
		t.Fatalf("buildWhere2Condition contains_all: %v", err)
	}
	if !strings.Contains(cond, "NOT EXISTS") {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}

	cond, args, inc, err = buildWhere2Condition(sqldb.SQLite, protosql.SQL_QUESTION, 2, field, protodb.WhereOperator_WOP_LEN_GTE, "2")
	if err != nil {
		t.Fatalf("buildWhere2Condition len_gte: %v", err)
	}
	if !strings.Contains(cond, "json_array_length") {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != int64(2) {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_RepeatedScalar_PostgresLenLTE(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("ResultColumnNames")

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 3, field, protodb.WhereOperator_WOP_LEN_LTE, "5")
	if err != nil {
		t.Fatalf("buildWhere2Condition len_lte: %v", err)
	}
	if cond != "cardinality(ResultColumnNames) <= $3" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != int64(5) {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_RepeatedScalar_PostgresOverlap(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("ResultColumnNames")

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, field, protodb.WhereOperator_WOP_OVERLAP, "[\"a\",\"b\"]")
	if err != nil {
		t.Fatalf("buildWhere2Condition overlap: %v", err)
	}
	if cond != "ResultColumnNames && $1" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_LenParseIntError(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("ResultColumnNames")
	if _, _, _, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, field, protodb.WhereOperator_WOP_LEN_GT, "x"); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, _, err := buildWhere2Condition(sqldb.SQLite, protosql.SQL_QUESTION, 1, field, protodb.WhereOperator_WOP_LEN_LT, "x"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWhere2Condition_RepeatedMessage_UnsupportedOp(t *testing.T) {
	_, _, _, subsField, _ := buildTestArrayDescriptors(t)
	if _, _, _, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, subsField, protodb.WhereOperator_WOP_OVERLAP, "[]"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildWhere2Condition_NonListField_ScalarOps(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("TableName")
	if field == nil {
		t.Fatalf("expected field")
	}

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, field, protodb.WhereOperator_WOP_EQ, "abc")
	if err != nil {
		t.Fatalf("buildWhere2Condition: %v", err)
	}
	if cond != "TableName = $1" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != "abc" {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}

	cond, args, inc, err = buildWhere2Condition(sqldb.SQLite, protosql.SQL_QUESTION, 1, field, protodb.WhereOperator_WOP_LIKE, "%ab%")
	if err != nil {
		t.Fatalf("buildWhere2Condition: %v", err)
	}
	if cond != "TableName LIKE ?" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != "%ab%" {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_NonListField_GT(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("TableName")

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 7, field, protodb.WhereOperator_WOP_GT, "x")
	if err != nil {
		t.Fatalf("buildWhere2Condition: %v", err)
	}
	if cond != "TableName > $7" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != "x" {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_RepeatedMessage_PostgresContainsAll(t *testing.T) {
	_, _, _, subsField, _ := buildTestArrayDescriptors(t)
	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, subsField, protodb.WhereOperator_WOP_CONTAINS_ALL, "[{\"name\":\"a\"}]")
	if err != nil {
		t.Fatalf("buildWhere2Condition contains_all: %v", err)
	}
	if cond != "subs @> $1::jsonb" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 {
		t.Fatalf("unexpected args/inc: %#v %d", args, inc)
	}
}

func TestBuildWhere2Condition_UnknownDialect_Error(t *testing.T) {
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	field := msgDesc.Fields().ByName("ResultColumnNames")
	_, _, _, err := buildWhere2Condition(sqldb.Unknown, protosql.SQL_QUESTION, 1, field, protodb.WhereOperator_WOP_CONTAINS, "a")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseScalarString_Errors(t *testing.T) {
	if _, err := parseScalarString(protoreflect.BoolKind, "notabool"); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := parseScalarString(protoreflect.Int64Kind, "nan"); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := parseScalarString(protoreflect.Uint64Kind, "-1"); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := parseScalarString(protoreflect.DoubleKind, "nanx"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseScalarJSONArray_ErrorInvalidJSON(t *testing.T) {
	if _, err := parseScalarJSONArray(protoreflect.StringKind, "not-json"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLenOpToSql_Default(t *testing.T) {
	if lenOpToSql(protodb.WhereOperator_WOP_UNKNOWN) != "" {
		t.Fatalf("expected empty")
	}
}
