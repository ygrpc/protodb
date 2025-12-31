package crud

import (
	"encoding/json"
	"testing"

	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func buildTestArrayDescriptors(t *testing.T) (msgDesc protoreflect.MessageDescriptor, u64Field protoreflect.FieldDescriptor, strsField protoreflect.FieldDescriptor, subsField protoreflect.FieldDescriptor, subMsgDesc protoreflect.MessageDescriptor) {
	t.Helper()

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("test_array.proto"),
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
						Name:   strPtr("u64s"),
						Number: int32Ptr(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
					},
					{
						Name:   strPtr("strs"),
						Number: int32Ptr(2),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
					{
						Name:     strPtr("subs"),
						Number:   int32Ptr(3),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
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

	subMsgDesc = fd.Messages().ByName("SubMsg")
	msgDesc = fd.Messages().ByName("TestMsg")
	u64Field = msgDesc.Fields().ByName("u64s")
	strsField = msgDesc.Fields().ByName("strs")
	subsField = msgDesc.Fields().ByName("subs")
	return msgDesc, u64Field, strsField, subsField, subMsgDesc
}

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }

func TestEncodeSQLArg_RepeatedUint64_PostgresConvert(t *testing.T) {
	_, u64Field, _, _, _ := buildTestArrayDescriptors(t)
	v, err := EncodeSQLArg(u64Field, sqldb.Postgres, []uint64{1, 2})
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

func TestEncodeSQLArg_RepeatedString_SQLiteJSON(t *testing.T) {
	_, _, strsField, _, _ := buildTestArrayDescriptors(t)
	v, err := EncodeSQLArg(strsField, sqldb.SQLite, []string{"a", "b"})
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Fatalf("unexpected arr: %#v", arr)
	}
}

func TestEncodeSQLArg_RepeatedMessage_JSONArray(t *testing.T) {
	_, _, _, subsField, subMsgDesc := buildTestArrayDescriptors(t)
	m1 := dynamicpb.NewMessage(subMsgDesc)
	m1.ProtoReflect().Set(subMsgDesc.Fields().ByName("name"), protoreflect.ValueOfString("a"))
	m2 := dynamicpb.NewMessage(subMsgDesc)
	m2.ProtoReflect().Set(subMsgDesc.Fields().ByName("name"), protoreflect.ValueOfString("b"))

	v, err := EncodeSQLArg(subsField, sqldb.Postgres, []*dynamicpb.Message{m1, m2})
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
	if len(raws) != 2 {
		t.Fatalf("expected 2 elems, got %d", len(raws))
	}
}
