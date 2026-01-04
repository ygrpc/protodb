package crud

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func boolPtr(b bool) *bool { return &b }

func buildTestMapDescriptors(t *testing.T) (msgDesc protoreflect.MessageDescriptor, mInt64Str protoreflect.FieldDescriptor, mStrSub protoreflect.FieldDescriptor, mBoolI32 protoreflect.FieldDescriptor, subMsgDesc protoreflect.MessageDescriptor) {
	t.Helper()

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("test_map.proto"),
		Package: strPtr("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("SubMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("name"),
						Number: int32Ptr(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
			{
				Name: strPtr("TestMsg"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name:    strPtr("MInt64StrEntry"),
						Options: &descriptorpb.MessageOptions{MapEntry: boolPtr(true)},
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("key"),
								Number: int32Ptr(1),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
							},
							{
								Name:   strPtr("value"),
								Number: int32Ptr(2),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
					{
						Name:    strPtr("MStrSubEntry"),
						Options: &descriptorpb.MessageOptions{MapEntry: boolPtr(true)},
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("key"),
								Number: int32Ptr(1),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
							{
								Name:     strPtr("value"),
								Number:   int32Ptr(2),
								Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
								TypeName: strPtr(".test.SubMsg"),
							},
						},
					},
					{
						Name:    strPtr("MBoolI32Entry"),
						Options: &descriptorpb.MessageOptions{MapEntry: boolPtr(true)},
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("key"),
								Number: int32Ptr(1),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
							},
							{
								Name:   strPtr("value"),
								Number: int32Ptr(2),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
							},
						},
					},
				},
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     strPtr("m_int64_str"),
						Number:   int32Ptr(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: strPtr(".test.TestMsg.MInt64StrEntry"),
					},
					{
						Name:     strPtr("m_str_sub"),
						Number:   int32Ptr(2),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: strPtr(".test.TestMsg.MStrSubEntry"),
					},
					{
						Name:     strPtr("m_bool_i32"),
						Number:   int32Ptr(3),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: strPtr(".test.TestMsg.MBoolI32Entry"),
					},
				},
			},
		},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}

	subMsgDesc = fd.Messages().ByName("SubMsg")
	msgDesc = fd.Messages().ByName("TestMsg")
	mInt64Str = msgDesc.Fields().ByName("m_int64_str")
	mStrSub = msgDesc.Fields().ByName("m_str_sub")
	mBoolI32 = msgDesc.Fields().ByName("m_bool_i32")
	return msgDesc, mInt64Str, mStrSub, mBoolI32, subMsgDesc
}

func TestEncodeSQLArg_MapScalar_JSON(t *testing.T) {
	_, mInt64Str, _, _, _ := buildTestMapDescriptors(t)

	v, err := EncodeSQLArg(mInt64Str, sqldb.Postgres, map[int64]string{1: "a", 2: "b"})
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["1"] != "a" || got["2"] != "b" {
		t.Fatalf("unexpected map: %#v", got)
	}
}

func TestEncodeSQLArg_MapMessage_JSON(t *testing.T) {
	_, _, mStrSub, _, subMsgDesc := buildTestMapDescriptors(t)
	m := map[string]any{}

	sub := dynamicpb.NewMessage(subMsgDesc)
	sub.ProtoReflect().Set(subMsgDesc.Fields().ByName("name"), protoreflect.ValueOfString("x"))
	m["k"] = sub

	v, err := EncodeSQLArg(mStrSub, sqldb.Postgres, m)
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	if !strings.Contains(s, "\"name\":\"x\"") {
		t.Fatalf("unexpected json: %s", s)
	}
}

func TestSetProtoMsgField_MapScalar_FromJSON(t *testing.T) {
	msgDesc, mInt64Str, _, _, _ := buildTestMapDescriptors(t)
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, mInt64Str, "{\"1\":\"a\",\"2\":\"b\"}"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	mp := msg.ProtoReflect().Get(mInt64Str).Map()
	if mp.Len() != 2 {
		t.Fatalf("expected len 2, got %d", mp.Len())
	}
	if mp.Get(protoreflect.ValueOfInt64(1).MapKey()).String() != "a" {
		t.Fatalf("unexpected value for key 1")
	}
	if mp.Get(protoreflect.ValueOfInt64(2).MapKey()).String() != "b" {
		t.Fatalf("unexpected value for key 2")
	}
}

func TestSetProtoMsgField_MapBoolKey_FromJSON(t *testing.T) {
	msgDesc, _, _, mBoolI32, _ := buildTestMapDescriptors(t)
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, mBoolI32, "{\"true\":1,\"0\":2}"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	mp := msg.ProtoReflect().Get(mBoolI32).Map()
	if mp.Len() != 2 {
		t.Fatalf("expected len 2, got %d", mp.Len())
	}
	if mp.Get(protoreflect.ValueOfBool(true).MapKey()).Int() != 1 {
		t.Fatalf("unexpected value for key true")
	}
	if mp.Get(protoreflect.ValueOfBool(false).MapKey()).Int() != 2 {
		t.Fatalf("unexpected value for key false")
	}
}

func TestSetProtoMsgField_MapMessage_FromJSON(t *testing.T) {
	msgDesc, _, mStrSub, _, subMsgDesc := buildTestMapDescriptors(t)
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, mStrSub, "{\"k\":{\"name\":\"x\"}}"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	mp := msg.ProtoReflect().Get(mStrSub).Map()
	mv := mp.Get(protoreflect.ValueOfString("k").MapKey()).Message()
	nameField := subMsgDesc.Fields().ByName("name")
	if mv.Get(nameField).String() != "x" {
		t.Fatalf("unexpected nested name")
	}
}

func TestBuildWhere2Condition_MapOperators_AllDialects(t *testing.T) {
	_, mInt64Str, _, _, _ := buildTestMapDescriptors(t)

	cond, args, inc, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, mInt64Str, protodb.WhereOperator_WOP_HAS_KEY, "1")
	if err != nil {
		t.Fatalf("pg has_key: %v", err)
	}
	if cond != "m_int64_str ? $1" {
		t.Fatalf("unexpected pg has_key cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != "1" {
		t.Fatalf("unexpected pg has_key args/inc: %#v %d", args, inc)
	}

	cond, args, inc, err = buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 2, mInt64Str, protodb.WhereOperator_WOP_CONTAINS, "{\"1\":\"a\"}")
	if err != nil {
		t.Fatalf("pg contains: %v", err)
	}
	if cond != "m_int64_str @> $2::jsonb" {
		t.Fatalf("unexpected pg contains cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 {
		t.Fatalf("unexpected pg contains args/inc: %#v %d", args, inc)
	}

	cond, args, inc, err = buildWhere2Condition(sqldb.SQLite, protosql.SQL_QUESTION, 1, mInt64Str, protodb.WhereOperator_WOP_HAS_KEY, "1")
	if err != nil {
		t.Fatalf("sqlite has_key: %v", err)
	}
	if !strings.Contains(cond, "json_each") {
		t.Fatalf("unexpected sqlite has_key cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != "1" {
		t.Fatalf("unexpected sqlite has_key args/inc: %#v %d", args, inc)
	}

	cond, args, inc, err = buildWhere2Condition(sqldb.SQLite, protosql.SQL_QUESTION, 2, mInt64Str, protodb.WhereOperator_WOP_CONTAINS, "{\"1\":\"a\"}")
	if err != nil {
		t.Fatalf("sqlite contains: %v", err)
	}
	if !strings.Contains(cond, "NOT EXISTS") {
		t.Fatalf("unexpected sqlite contains cond: %s", cond)
	}
	if inc != 1 || len(args) != 1 || args[0] != "{\"1\":\"a\"}" {
		t.Fatalf("unexpected sqlite contains args/inc: %#v %d", args, inc)
	}

	cond, _, _, err = buildWhere2Condition(sqldb.Mysql, protosql.SQL_QUESTION, 1, mInt64Str, protodb.WhereOperator_WOP_HAS_KEY, "1")
	if err != nil {
		t.Fatalf("mysql has_key: %v", err)
	}
	if !strings.Contains(cond, "JSON_CONTAINS_PATH") {
		t.Fatalf("unexpected mysql has_key cond: %s", cond)
	}

	cond, _, _, err = buildWhere2Condition(sqldb.Mysql, protosql.SQL_QUESTION, 1, mInt64Str, protodb.WhereOperator_WOP_CONTAINS, "{\"1\":\"a\"}")
	if err != nil {
		t.Fatalf("mysql contains: %v", err)
	}
	if !strings.Contains(cond, "JSON_CONTAINS") {
		t.Fatalf("unexpected mysql contains cond: %s", cond)
	}
}

func TestBuildWhere2Condition_MapUnsupportedOp(t *testing.T) {
	_, mInt64Str, _, _, _ := buildTestMapDescriptors(t)
	if _, _, _, err := buildWhere2Condition(sqldb.Postgres, protosql.SQL_DOLLAR, 1, mInt64Str, protodb.WhereOperator_WOP_LEN_GT, "1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseMapDescriptors_IsMap(t *testing.T) {
	_, mInt64Str, mStrSub, mBoolI32, _ := buildTestMapDescriptors(t)
	if !mInt64Str.IsMap() || !mStrSub.IsMap() || !mBoolI32.IsMap() {
		t.Fatalf("expected fields to be map")
	}
	if mInt64Str.MapKey().Kind() != protoreflect.Int64Kind {
		t.Fatalf("unexpected key kind: %v", mInt64Str.MapKey().Kind())
	}
	if mInt64Str.MapValue().Kind() != protoreflect.StringKind {
		t.Fatalf("unexpected value kind: %v", mInt64Str.MapValue().Kind())
	}
	if mStrSub.MapValue().Kind() != protoreflect.MessageKind {
		t.Fatalf("unexpected msg value kind: %v", mStrSub.MapValue().Kind())
	}
	if mBoolI32.MapKey().Kind() != protoreflect.BoolKind {
		t.Fatalf("unexpected bool key kind: %v", mBoolI32.MapKey().Kind())
	}
}

func TestEncodeSQLArg_MapNil_ReturnsEmptyObject(t *testing.T) {
	_, mInt64Str, _, _, _ := buildTestMapDescriptors(t)
	var mv map[int64]string
	v, err := EncodeSQLArg(mInt64Str, sqldb.Postgres, mv)
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, map[string]any{}) {
		t.Fatalf("unexpected map: %#v", got)
	}
}
