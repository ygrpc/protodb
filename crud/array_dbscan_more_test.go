package crud

import (
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestUnwrapScanVal(t *testing.T) {
	var a any = "x"
	if got := unwrapScanVal(&a); got != "x" {
		t.Fatalf("expected x, got %#v", got)
	}

	s := "y"
	if got := unwrapScanVal(&s); got != "y" {
		t.Fatalf("expected y, got %#v", got)
	}

	b := []byte("z")
	if got := unwrapScanVal(&b); string(got.([]byte)) != "z" {
		t.Fatalf("expected z, got %#v", got)
	}

	if got := unwrapScanVal(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestParsePGArrayLiteral_QuotesEscapesNULL(t *testing.T) {
	elems, err := parsePGArrayLiteral("{\"a,b\",\"c\",NULL,1}")
	if err != nil {
		t.Fatalf("parsePGArrayLiteral: %v", err)
	}
	if len(elems) != 4 {
		t.Fatalf("expected 4 elems, got %d", len(elems))
	}
	if elems[0] != "a,b" {
		t.Fatalf("unexpected elem0: %q", elems[0])
	}
	if elems[1] != "c" {
		t.Fatalf("unexpected elem1: %q", elems[1])
	}
	if elems[2] != "" {
		t.Fatalf("expected NULL->empty string, got %q", elems[2])
	}
	if elems[3] != "1" {
		t.Fatalf("unexpected elem3: %q", elems[3])
	}
}

func TestSetProtoMsgField_RepeatedScalar_FromSliceTypes(t *testing.T) {
	msgDesc, u64Field, strsField, _, _ := buildTestArrayDescriptors(t)

	msg := dynamicpb.NewMessage(msgDesc)
	if err := SetProtoMsgField(msg, u64Field, []int64{1, 2}); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	list := msg.ProtoReflect().Get(u64Field).List()
	if list.Len() != 2 || list.Get(0).Uint() != 1 || list.Get(1).Uint() != 2 {
		t.Fatalf("unexpected list: %v", list)
	}

	msg2 := dynamicpb.NewMessage(msgDesc)
	if err := SetProtoMsgField(msg2, strsField, []string{"a", "b"}); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	l2 := msg2.ProtoReflect().Get(strsField).List()
	if l2.Len() != 2 || l2.Get(0).String() != "a" || l2.Get(1).String() != "b" {
		t.Fatalf("unexpected list: %v", l2)
	}
}

func TestSetProtoMsgField_RepeatedBool_FromJSON(t *testing.T) {
	// build a message with repeated bool
	fdp := buildTestFileDescriptorProtoForScalarList(t, "bools", descriptorTypeBool)
	msgDesc := fdp.Messages().ByName("TestMsg")
	field := msgDesc.Fields().ByName("bools")
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, field, "[true,false,1,0,\"true\",\"0\"]"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	list := msg.ProtoReflect().Get(field).List()
	if list.Len() != 6 {
		t.Fatalf("expected 6, got %d", list.Len())
	}
	if list.Get(0).Bool() != true || list.Get(1).Bool() != false || list.Get(2).Bool() != true || list.Get(3).Bool() != false || list.Get(4).Bool() != true || list.Get(5).Bool() != false {
		t.Fatalf("unexpected bool list")
	}
}

func TestScalarElemToProtoreflectValue_FloatAndBytes(t *testing.T) {
	v, err := scalarElemToProtoreflectValue(protoreflect.DoubleKind, "1.25")
	if err != nil {
		t.Fatalf("scalarElemToProtoreflectValue: %v", err)
	}
	if v.Float() != 1.25 {
		t.Fatalf("unexpected float: %v", v.Float())
	}

	b64 := "AQID"
	vv, err := scalarElemToProtoreflectValue(protoreflect.BytesKind, b64)
	if err != nil {
		t.Fatalf("scalarElemToProtoreflectValue bytes: %v", err)
	}
	if got := vv.Bytes(); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("unexpected bytes: %#v", got)
	}
}

// Helpers to create a dynamic message with one repeated scalar field.

type scalarListType int

const (
	descriptorTypeBool scalarListType = iota
)

func buildTestFileDescriptorProtoForScalarList(t *testing.T, fieldName string, typ scalarListType) protoreflect.FileDescriptor {
	t.Helper()

	var fdType descriptorpb.FieldDescriptorProto_Type
	switch typ {
	case descriptorTypeBool:
		fdType = descriptorpb.FieldDescriptorProto_TYPE_BOOL
	default:
		fdType = descriptorpb.FieldDescriptorProto_TYPE_BOOL
	}

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("test_scalar_list.proto"),
		Package: strPtr("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TestMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr(fieldName),
						Number: int32Ptr(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   fdType.Enum(),
					},
				},
			},
		},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd
}
