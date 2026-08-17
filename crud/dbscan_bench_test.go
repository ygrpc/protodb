package crud

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/proto"
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

var benchScanColumnNames = []string{"id", "name", "active", "score", "amount", "count", "flags", "status"}

var benchScanRowVals = []any{
	int64(42),
	"hello",
	true,
	float64(3.14),
	float64(2.718),
	int64(100),
	int64(7),
	int64(1),
}

type benchmarkRowScanner interface {
	Scan(rows *sql.Rows, msg proto.Message) error
}

type benchmarkPreallocatedScanner struct {
	fieldDescs []protoreflect.FieldDescriptor
	rowVals    []any
}

func newBenchmarkPreallocatedScanner(columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) *benchmarkPreallocatedScanner {
	fieldDescs := make([]protoreflect.FieldDescriptor, len(columnNames))
	rowVals := make([]any, len(columnNames))
	for i, columnName := range columnNames {
		fieldDesc := msgFieldsMap[strings.ToLower(columnName)]
		fieldDescs[i] = fieldDesc
		if fieldDesc == nil {
			rowVals[i] = new(any)
			continue
		}
		rowVals[i] = allocScanDest(fieldDesc)
	}
	return &benchmarkPreallocatedScanner{fieldDescs: fieldDescs, rowVals: rowVals}
}

func (s *benchmarkPreallocatedScanner) Scan(rows *sql.Rows, msg proto.Message) error {
	if err := rows.Scan(s.rowVals...); err != nil {
		return err
	}
	for i, fieldDesc := range s.fieldDescs {
		if fieldDesc == nil {
			continue
		}
		if err := setProtoMsgFieldDirect(msg, fieldDesc, s.rowVals[i]); err != nil {
			if !canFallbackDirectError(err) {
				return err
			}
			if err := SetProtoMsgField(msg, fieldDesc, unwrapScanVal(s.rowVals[i])); err != nil {
				return err
			}
		}
	}
	return nil
}

func BenchmarkDbRowScannerAssign_MapLookup(b *testing.B) {
	fields := benchMsgDesc.Fields()
	msgFieldsMap := map[string]protoreflect.FieldDescriptor{
		"id":     fields.ByName("id"),
		"name":   fields.ByName("name"),
		"active": fields.ByName("active"),
		"score":  fields.ByName("score"),
		"amount": fields.ByName("amount"),
		"count":  fields.ByName("count"),
		"flags":  fields.ByName("flags"),
		"status": fields.ByName("status"),
	}
	msg := dynamicpb.NewMessage(benchMsgDesc)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, columnName := range benchScanColumnNames {
			fieldDesc := msgFieldsMap[strings.ToLower(columnName)]
			if err := setProtoMsgFieldDirect(msg, fieldDesc, benchScanRowVals[j]); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkDbRowScannerAssign_PrecomputedDescriptors(b *testing.B) {
	fields := benchMsgDesc.Fields()
	fieldDescs := []protoreflect.FieldDescriptor{
		fields.ByName("id"),
		fields.ByName("name"),
		fields.ByName("active"),
		fields.ByName("score"),
		fields.ByName("amount"),
		fields.ByName("count"),
		fields.ByName("flags"),
		fields.ByName("status"),
	}
	msg := dynamicpb.NewMessage(benchMsgDesc)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, fieldDesc := range fieldDescs {
			if err := setProtoMsgFieldDirect(msg, fieldDesc, benchScanRowVals[j]); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkDbRowScannerScan(b *testing.B) {
	b.Run("PreallocatedDest", func(b *testing.B) {
		benchmarkDbRowScannerScan(b, func(_ *sql.Rows, _ proto.Message, columns []string, fields map[string]protoreflect.FieldDescriptor) (benchmarkRowScanner, error) {
			return newBenchmarkPreallocatedScanner(columns, fields), nil
		})
	})
	b.Run("ProtoFieldReceiver", func(b *testing.B) {
		benchmarkDbRowScannerScan(b, func(rows *sql.Rows, msg proto.Message, columns []string, fields map[string]protoreflect.FieldDescriptor) (benchmarkRowScanner, error) {
			return NewDbRowScanner(rows, msg, columns, fields)
		})
	})
}

func benchmarkDbRowScannerScan(b *testing.B, newScanner func(*sql.Rows, proto.Message, []string, map[string]protoreflect.FieldDescriptor) (benchmarkRowScanner, error)) {
	columns := []string{"id", "name", "active", "score", "amount", "count", "flags", "hash", "data"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		db, mock, err := sqlmock.New()
		if err != nil {
			b.Fatal(err)
		}
		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows(columns).
				AddRow(int64(42), "hello", true, float64(3.14), float64(2.718), int64(100), int64(7), int64(12345), []byte("raw")).
				AddRow(int64(43), "world", false, float64(6.28), float64(1.618), int64(200), int64(8), int64(99999), []byte("data")),
		)
		mock.ExpectClose()
		rows, err := db.Query("SELECT")
		if err != nil {
			b.Fatal(err)
		}
		if !rows.Next() {
			b.Fatal("expected row")
		}
		msg := dynamicpb.NewMessage(benchMsgDesc)
		msgFieldsMap := pdbutil.BuildMsgFieldsMap(columns, benchMsgDesc.Fields(), true)
		scanner, err := newScanner(rows, msg, columns, msgFieldsMap)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		if err := scanner.Scan(rows, msg); err != nil {
			b.Fatal(err)
		}
		proto.Reset(msg)
		if !rows.Next() {
			b.Fatal("expected second row")
		}
		if err := scanner.Scan(rows, msg); err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		if err := rows.Close(); err != nil {
			b.Fatal(err)
		}
		if err := db.Close(); err != nil {
			b.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			b.Fatal(err)
		}
	}
}
