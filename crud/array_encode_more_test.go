package crud

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestEncodeSQLArg_NullStringInvalid_Passthrough(t *testing.T) {
	_, _, strsField, _, _ := buildTestArrayDescriptors(t)

	ns := sql.NullString{String: "", Valid: false}
	v, err := EncodeSQLArg(strsField, sqldb.SQLite, ns)
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	out, ok := v.(sql.NullString)
	if !ok {
		t.Fatalf("expected sql.NullString, got %T", v)
	}
	if out.Valid {
		t.Fatalf("expected invalid NullString")
	}
}

func TestEncodeSQLArg_RepeatedNilSlice_NormalizeEmpty(t *testing.T) {
	_, _, strsField, _, _ := buildTestArrayDescriptors(t)

	var s []string
	v, err := EncodeSQLArg(strsField, sqldb.Postgres, s)
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		t.Fatalf("expected slice, got %T", v)
	}
	if rv.IsNil() {
		t.Fatalf("expected non-nil slice")
	}
	if rv.Len() != 0 {
		t.Fatalf("expected len 0, got %d", rv.Len())
	}
}

func TestEncodeSQLArg_RepeatedUint32_PostgresConvert(t *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("test_u32_array.proto"),
		Package: strPtr("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TestMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("u32s"),
						Number: int32Ptr(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum(),
					},
				},
			},
		},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	msgDesc := fd.Messages().ByName("TestMsg")
	field := msgDesc.Fields().ByName("u32s")

	v, err := EncodeSQLArg(field, sqldb.Postgres, []uint32{1, 2})
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	out, ok := v.([]int64)
	if !ok {
		t.Fatalf("expected []int64, got %T", v)
	}
	if len(out) != 2 || out[0] != 1 || out[1] != 2 {
		t.Fatalf("unexpected out: %#v", out)
	}
}

func TestEncodeSQLArg_RepeatedMessage_NilElemSkipped(t *testing.T) {
	_, _, _, subsField, subMsgDesc := buildTestArrayDescriptors(t)

	m1 := dynamicpb.NewMessage(subMsgDesc)
	m1.ProtoReflect().Set(subMsgDesc.Fields().ByName("name"), protoreflect.ValueOfString("a"))

	v, err := EncodeSQLArg(subsField, sqldb.Postgres, []*dynamicpb.Message{nil, m1})
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	var raws []json.RawMessage
	if err := json.Unmarshal([]byte(s), &raws); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(raws) != 1 {
		t.Fatalf("expected 1 elem, got %d", len(raws))
	}
}

func TestEncodeSQLArg_RepeatedMessage_BadType(t *testing.T) {
	_, _, _, subsField, _ := buildTestArrayDescriptors(t)
	_, err := EncodeSQLArg(subsField, sqldb.Postgres, "not-slice")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestEncodeSQLArg_MessageField_JSON(t *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("test_msg_field.proto"),
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
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     strPtr("sub"),
						Number:   int32Ptr(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: strPtr(".test.SubMsg"),
					},
				},
			},
		},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	subDesc := fd.Messages().ByName("SubMsg")
	testDesc := fd.Messages().ByName("TestMsg")
	field := testDesc.Fields().ByName("sub")

	sub := dynamicpb.NewMessage(subDesc)
	sub.ProtoReflect().Set(subDesc.Fields().ByName("name"), protoreflect.ValueOfString("x"))

	v, err := EncodeSQLArg(field, sqldb.Postgres, sub)
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
