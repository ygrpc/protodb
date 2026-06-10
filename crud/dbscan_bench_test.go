package crud

import (
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// buildBenchMsgDesc 构建一个包含多种标量字段的测试消息描述符
func buildBenchMsgDesc() protoreflect.MessageDescriptor {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("bench.proto"),
		Package: strPtr("bench"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("BenchMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("id"), Number: int32Ptr(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
					{Name: strPtr("name"), Number: int32Ptr(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
					{Name: strPtr("active"), Number: int32Ptr(3), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
					{Name: strPtr("score"), Number: int32Ptr(4), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()},
					{Name: strPtr("amount"), Number: int32Ptr(5), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()},
					{Name: strPtr("count"), Number: int32Ptr(6), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
					{Name: strPtr("flags"), Number: int32Ptr(7), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum()},
					{Name: strPtr("hash"), Number: int32Ptr(8), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum()},
					{Name: strPtr("data"), Number: int32Ptr(9), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()},
					{Name: strPtr("status"), Number: int32Ptr(10), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
				},
			},
		},
		Syntax: strPtr("proto3"),
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		panic(err)
	}
	return fd.Messages().ByName("BenchMsg")
}

var benchMsgDesc = buildBenchMsgDesc()

// BenchmarkSetProtoMsgField_Old 测试旧路径（经过 pdbutil.SetField 反射）
func BenchmarkSetProtoMsgField_Old(b *testing.B) {
	fields := benchMsgDesc.Fields()
	for i := 0; i < b.N; i++ {
		msg := dynamicpb.NewMessage(benchMsgDesc)
		_ = SetProtoMsgField(msg, fields.ByName("id"), int64(42))
		_ = SetProtoMsgField(msg, fields.ByName("name"), "hello")
		_ = SetProtoMsgField(msg, fields.ByName("active"), true)
		_ = SetProtoMsgField(msg, fields.ByName("score"), float64(3.14))
		_ = SetProtoMsgField(msg, fields.ByName("amount"), float64(2.718))
		_ = SetProtoMsgField(msg, fields.ByName("count"), int64(100))
		_ = SetProtoMsgField(msg, fields.ByName("flags"), int64(7))
		_ = SetProtoMsgField(msg, fields.ByName("hash"), int64(12345))
		_ = SetProtoMsgField(msg, fields.ByName("data"), []byte("raw"))
		_ = SetProtoMsgField(msg, fields.ByName("status"), int64(1))
	}
}

// BenchmarkSetProtoMsgField_Direct 测试新路径（直接 protoreflect，无反射）
func BenchmarkSetProtoMsgField_Direct(b *testing.B) {
	fields := benchMsgDesc.Fields()
	for i := 0; i < b.N; i++ {
		msg := dynamicpb.NewMessage(benchMsgDesc)
		_ = setProtoMsgFieldDirect(msg, fields.ByName("id"), int64(42))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("name"), "hello")
		_ = setProtoMsgFieldDirect(msg, fields.ByName("active"), true)
		_ = setProtoMsgFieldDirect(msg, fields.ByName("score"), float64(3.14))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("amount"), float64(2.718))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("count"), int64(100))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("flags"), int64(7))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("hash"), int64(12345))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("data"), []byte("raw"))
		_ = setProtoMsgFieldDirect(msg, fields.ByName("status"), int64(1))
	}
}
