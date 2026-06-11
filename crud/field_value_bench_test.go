package crud

import (
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var benchFieldValueSink any

func benchFieldValueDescriptors() []protoreflect.FieldDescriptor {
	fields := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor().Fields()
	return []protoreflect.FieldDescriptor{
		fields.ByName("SchemeName"),
		fields.ByName("TableName"),
		fields.ByName("Limit"),
		fields.ByName("Offset"),
		fields.ByName("PreferBatchSize"),
		fields.ByName("MsgFormat"),
	}
}

func BenchmarkGetSQLFieldValue_ReflectFieldByName(b *testing.B) {
	msg := &protodb.TableQueryReq{
		SchemeName:      "public",
		TableName:       "User",
		Limit:           100,
		Offset:          200,
		PreferBatchSize: 50,
		MsgFormat:       1,
	}
	fields := benchFieldValueDescriptors()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, field := range fields {
			val, err := pdbutil.GetField(msg, string(field.Name()))
			if err != nil {
				b.Fatal(err)
			}
			benchFieldValueSink = val
		}
	}
}

func BenchmarkGetSQLFieldValue_ProtoReflectScalar(b *testing.B) {
	msg := &protodb.TableQueryReq{
		SchemeName:      "public",
		TableName:       "User",
		Limit:           100,
		Offset:          200,
		PreferBatchSize: 50,
		MsgFormat:       1,
	}
	fields := benchFieldValueDescriptors()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, field := range fields {
			val, err := getSQLFieldValue(msg, field)
			if err != nil {
				b.Fatal(err)
			}
			benchFieldValueSink = val
		}
	}
}
