package pdbutil

import (
	"testing"

	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestIsZeroValue(t *testing.T) {
	type args struct {
		val interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "zero int",
			args: args{
				val: 0,
			},
			want: true,
		},
		{
			name: "zero string",
			args: args{
				val: "",
			},
			want: true,
		},
		//dobule 0.0
		{
			name: "zero double",
			args: args{
				val: 0.0,
			},
			want: true,
		},
		//float 0.0
		{
			name: "zero float",
			args: args{
				val: float32(0.0),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZeroValue(tt.args.val); got != tt.want {
				t.Errorf("IsZeroValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

var isZeroValueSink bool

func BenchmarkIsZeroValueScalars(b *testing.B) {
	vals := []any{
		0,
		int32(0),
		int64(1),
		uint32(0),
		uint64(1),
		float32(0),
		float64(1),
		"",
		"abc",
		false,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, val := range vals {
			isZeroValueSink = IsZeroValue(val)
		}
	}
}

var buildMsgFieldsMapSink map[string]protoreflect.FieldDescriptor

func BenchmarkBuildMsgFieldsMapAllFields(b *testing.B) {
	fields := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor().Fields()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buildMsgFieldsMapSink = BuildMsgFieldsMap(nil, fields, true)
	}
}

func BenchmarkBuildMsgFieldsMapSelectedFields(b *testing.B) {
	fields := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor().Fields()
	fieldNames := []string{"SchemeName", "TableName", "Limit", "Offset"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buildMsgFieldsMapSink = BuildMsgFieldsMap(fieldNames, fields, true)
	}
}

func TestGetPDBCacheSeparatesFilesWithSameFullName(t *testing.T) {
	fieldWithoutOption := buildPDBTestField(t, "pdb_cache_a.proto", false)
	fieldWithOption := buildPDBTestField(t, "pdb_cache_b.proto", true)

	pdb, found := GetPDB(fieldWithoutOption)
	if found || pdb != EmptyPDB {
		t.Fatalf("GetPDB without option = (%#v, %v), want EmptyPDB,false", pdb, found)
	}

	pdb, found = GetPDB(fieldWithOption)
	if !found || !pdb.IsPrimary() {
		t.Fatalf("GetPDB with option = (%#v, %v), want primary,true", pdb, found)
	}

	pdb, found = GetPDB(fieldWithoutOption)
	if found || pdb != EmptyPDB {
		t.Fatalf("GetPDB without option after cached peer = (%#v, %v), want EmptyPDB,false", pdb, found)
	}
}

func TestGetPDBCacheSeparatesDescriptorInstancesWithSameName(t *testing.T) {
	fieldWithoutOption := buildPDBTestField(t, "pdb_cache_same.proto", false)
	fieldWithOption := buildPDBTestField(t, "pdb_cache_same.proto", true)

	pdb, found := GetPDB(fieldWithoutOption)
	if found || pdb != EmptyPDB {
		t.Fatalf("GetPDB without option = (%#v, %v), want EmptyPDB,false", pdb, found)
	}

	pdb, found = GetPDB(fieldWithOption)
	if !found || !pdb.IsPrimary() {
		t.Fatalf("GetPDB with option from same descriptor name = (%#v, %v), want primary,true", pdb, found)
	}

	pdb, found = GetPDB(fieldWithoutOption)
	if found || pdb != EmptyPDB {
		t.Fatalf("GetPDB without option after same-name peer = (%#v, %v), want EmptyPDB,false", pdb, found)
	}
}

func TestBuildMsgFieldsMapCacheSeparatesFilesWithSameFullName(t *testing.T) {
	msgA := buildFieldMapTestMessage(t, "field_map_cache_a.proto", "id")
	msgB := buildFieldMapTestMessage(t, "field_map_cache_b.proto", "email")

	fieldsA := BuildMsgFieldsMap(nil, msgA.Fields(), true)
	if _, ok := fieldsA["id"]; !ok {
		t.Fatalf("fieldsA missing id: %#v", fieldsA)
	}
	if _, ok := fieldsA["email"]; ok {
		t.Fatalf("fieldsA unexpectedly contains email: %#v", fieldsA)
	}

	fieldsB := BuildMsgFieldsMap(nil, msgB.Fields(), true)
	if _, ok := fieldsB["email"]; !ok {
		t.Fatalf("fieldsB missing email: %#v", fieldsB)
	}
	if _, ok := fieldsB["id"]; ok {
		t.Fatalf("fieldsB unexpectedly contains id: %#v", fieldsB)
	}

	fieldsA = BuildMsgFieldsMap(nil, msgA.Fields(), true)
	if _, ok := fieldsA["id"]; !ok {
		t.Fatalf("fieldsA missing id after peer cache: %#v", fieldsA)
	}
	if _, ok := fieldsA["email"]; ok {
		t.Fatalf("fieldsA contains email after peer cache: %#v", fieldsA)
	}
}

func TestBuildMsgFieldsMapCacheSeparatesDescriptorInstancesWithSameName(t *testing.T) {
	msgA := buildFieldMapTestMessage(t, "field_map_cache_same.proto", "id")
	msgB := buildFieldMapTestMessage(t, "field_map_cache_same.proto", "email")

	fieldsA := BuildMsgFieldsMap(nil, msgA.Fields(), true)
	if _, ok := fieldsA["id"]; !ok {
		t.Fatalf("fieldsA missing id: %#v", fieldsA)
	}
	if _, ok := fieldsA["email"]; ok {
		t.Fatalf("fieldsA unexpectedly contains email: %#v", fieldsA)
	}

	fieldsB := BuildMsgFieldsMap(nil, msgB.Fields(), true)
	if _, ok := fieldsB["email"]; !ok {
		t.Fatalf("fieldsB missing email: %#v", fieldsB)
	}
	if _, ok := fieldsB["id"]; ok {
		t.Fatalf("fieldsB unexpectedly contains id: %#v", fieldsB)
	}

	fieldsA = BuildMsgFieldsMap(nil, msgA.Fields(), true)
	if _, ok := fieldsA["id"]; !ok {
		t.Fatalf("fieldsA missing id after same-name peer cache: %#v", fieldsA)
	}
	if _, ok := fieldsA["email"]; ok {
		t.Fatalf("fieldsA contains email after same-name peer cache: %#v", fieldsA)
	}
}

var pdbSink *protodb.PDBField
var pdbFoundSink bool

func BenchmarkGetPDBCacheHit(b *testing.B) {
	field := buildPDBTestField(b, "pdb_cache_bench.proto", true)
	GetPDB(field)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pdbSink, pdbFoundSink = GetPDB(field)
	}
}

func BenchmarkGetPDBDirectExtensionRead(b *testing.B) {
	field := buildPDBTestField(b, "pdb_direct_bench.proto", true)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		opts := field.Options()
		if opts == nil || !proto.HasExtension(opts, protodb.E_Pdb) {
			pdbSink, pdbFoundSink = EmptyPDB, false
			continue
		}
		pdbSink, pdbFoundSink = proto.GetExtension(opts, protodb.E_Pdb).(*protodb.PDBField)
	}
}

func BenchmarkGetPDBCacheHitNoOption(b *testing.B) {
	field := buildPDBTestField(b, "pdb_cache_no_option_bench.proto", false)
	GetPDB(field)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pdbSink, pdbFoundSink = GetPDB(field)
	}
}

func BenchmarkGetPDBDirectNoOptionRead(b *testing.B) {
	field := buildPDBTestField(b, "pdb_direct_no_option_bench.proto", false)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		opts := field.Options()
		if opts == nil || !proto.HasExtension(opts, protodb.E_Pdb) {
			pdbSink, pdbFoundSink = EmptyPDB, false
			continue
		}
		pdbSink, pdbFoundSink = proto.GetExtension(opts, protodb.E_Pdb).(*protodb.PDBField)
	}
}

var primaryKeyFieldsSink map[string]protoreflect.FieldDescriptor
var uniqueFieldsSink map[string]*TuniqueConstraints

func BenchmarkGetPrimaryKeyFieldDescs(b *testing.B) {
	msgDesc := buildPDBTestMessage(b, "pdb_primary_bench.proto")
	fields := msgDesc.Fields()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		primaryKeyFieldsSink = GetPrimaryKeyFieldDescs(msgDesc, fields, false)
	}
}

func BenchmarkGetPrimaryKeyOrUniqueFieldDescs(b *testing.B) {
	msgDesc := buildPDBTestMessage(b, "pdb_unique_bench.proto")
	fields := msgDesc.Fields()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		uniqueFieldsSink = GetPrimaryKeyOrUniqueFieldDescs(msgDesc, fields, false)
	}
}

func buildPDBTestField(tb testing.TB, fileName string, withOption bool) protoreflect.FieldDescriptor {
	tb.Helper()

	field := &descriptorpb.FieldDescriptorProto{
		Name:   testStringPtr("id"),
		Number: testInt32Ptr(1),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
	}
	if withOption {
		opts := &descriptorpb.FieldOptions{}
		proto.SetExtension(opts, protodb.E_Pdb, &protodb.PDBField{Primary: true})
		field.Options = opts
	}

	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    testStringPtr(fileName),
		Package: testStringPtr("cachetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  testStringPtr("Msg"),
				Field: []*descriptorpb.FieldDescriptorProto{field},
			},
		},
		Syntax: testStringPtr("proto3"),
	}, nil)
	if err != nil {
		tb.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd.Messages().ByName("Msg").Fields().ByName("id")
}

func buildPDBTestMessage(tb testing.TB, fileName string) protoreflect.MessageDescriptor {
	tb.Helper()

	primaryOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(primaryOpts, protodb.E_Pdb, &protodb.PDBField{Primary: true})
	uniqueOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(uniqueOpts, protodb.E_Pdb, &protodb.PDBField{Unique: true})

	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    testStringPtr(fileName),
		Package: testStringPtr("cachetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: testStringPtr("Msg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:    testStringPtr("id"),
						Number:  testInt32Ptr(1),
						Label:   descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:    descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
						Options: primaryOpts,
					},
					{
						Name:    testStringPtr("email"),
						Number:  testInt32Ptr(2),
						Label:   descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:    descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Options: uniqueOpts,
					},
					{
						Name:   testStringPtr("name"),
						Number: testInt32Ptr(3),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
		},
		Syntax: testStringPtr("proto3"),
	}, nil)
	if err != nil {
		tb.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd.Messages().ByName("Msg")
}

func buildFieldMapTestMessage(tb testing.TB, fileName string, fieldName string) protoreflect.MessageDescriptor {
	tb.Helper()

	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    testStringPtr(fileName),
		Package: testStringPtr("cachetest"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: testStringPtr("Msg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   testStringPtr(fieldName),
						Number: testInt32Ptr(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
		},
		Syntax: testStringPtr("proto3"),
	}, nil)
	if err != nil {
		tb.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd.Messages().ByName("Msg")
}

func testStringPtr(s string) *string { return &s }

func testInt32Ptr(i int32) *int32 { return &i }
