package crud

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestSetProtoMsgField_RepeatedString_FromJSON(t *testing.T) {
	msgDesc, _, strsField, _, _ := buildTestArrayDescriptors(t)
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, strsField, "[\"a\",\"b\"]"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	list := msg.ProtoReflect().Get(strsField).List()
	if list.Len() != 2 {
		t.Fatalf("expected len 2, got %d", list.Len())
	}
	if list.Get(0).String() != "a" || list.Get(1).String() != "b" {
		t.Fatalf("unexpected list: %v %v", list.Get(0).String(), list.Get(1).String())
	}
}

func TestSetProtoMsgField_RepeatedUint64_FromPGArrayLiteral(t *testing.T) {
	msgDesc, u64Field, _, _, _ := buildTestArrayDescriptors(t)
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, u64Field, "{1,2}"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	list := msg.ProtoReflect().Get(u64Field).List()
	if list.Len() != 2 {
		t.Fatalf("expected len 2, got %d", list.Len())
	}
	if list.Get(0).Uint() != 1 || list.Get(1).Uint() != 2 {
		t.Fatalf("unexpected list: %d %d", list.Get(0).Uint(), list.Get(1).Uint())
	}
}

func TestSetProtoMsgField_RepeatedMessage_FromJSONArray(t *testing.T) {
	msgDesc, _, _, subsField, subMsgDesc := buildTestArrayDescriptors(t)
	msg := dynamicpb.NewMessage(msgDesc)

	if err := SetProtoMsgField(msg, subsField, "[{\"name\":\"a\"},{\"name\":\"b\"}]"); err != nil {
		t.Fatalf("SetProtoMsgField: %v", err)
	}
	list := msg.ProtoReflect().Get(subsField).List()
	if list.Len() != 2 {
		t.Fatalf("expected len 2, got %d", list.Len())
	}
	for i := 0; i < list.Len(); i++ {
		em := list.Get(i).Message()
		nameField := subMsgDesc.Fields().ByName("name")
		nameVal := em.Get(nameField)
		if nameField.Kind() != protoreflect.StringKind {
			t.Fatalf("unexpected kind %v", nameField.Kind())
		}
		_ = nameVal
	}
}
