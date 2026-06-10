package crud

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func buildPhase1ScanMsgDesc(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("phase1_scan.proto"),
		Package: strPtr("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("Phase1ScanMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: strPtr("id"), Number: int32Ptr(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
					{Name: strPtr("name"), Number: int32Ptr(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
					{Name: strPtr("active"), Number: int32Ptr(3), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
					{Name: strPtr("score"), Number: int32Ptr(4), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()},
					{Name: strPtr("data"), Number: int32Ptr(5), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()},
					{Name: strPtr("tags"), Number: int32Ptr(6), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
					{Name: strPtr("hash"), Number: int32Ptr(7), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum()},
				},
			},
		},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	return fd.Messages().ByName("Phase1ScanMsg")
}

func TestDbScan2ProtoMsg_NullValuesDoNotFailScan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	columns := []string{"id", "name", "active", "score", "data", "tags"}
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns).AddRow(nil, nil, nil, nil, nil, nil))

	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}

	msgDesc := buildPhase1ScanMsgDesc(t)
	msg := dynamicpb.NewMessage(msgDesc)
	msgFieldsMap := pdbutil.BuildMsgFieldsMap(columns, msgDesc.Fields(), true)
	if err := DbScan2ProtoMsg(rows, msg, columns, msgFieldsMap); err != nil {
		t.Fatalf("DbScan2ProtoMsg: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestDbScan2ProtoMsg_SetsTypedValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	columns := []string{"id", "name", "active", "score", "data", "tags"}
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns).AddRow(int64(7), "alice", true, float64(1.5), []byte("raw"), "[\"a\",\"b\"]"))

	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}

	msgDesc := buildPhase1ScanMsgDesc(t)
	msg := dynamicpb.NewMessage(msgDesc)
	msgFieldsMap := pdbutil.BuildMsgFieldsMap(columns, msgDesc.Fields(), true)
	if err := DbScan2ProtoMsg(rows, msg, columns, msgFieldsMap); err != nil {
		t.Fatalf("DbScan2ProtoMsg: %v", err)
	}

	pm := msg.ProtoReflect()
	if got := pm.Get(msgDesc.Fields().ByName("id")).Int(); got != 7 {
		t.Fatalf("id = %d", got)
	}
	if got := pm.Get(msgDesc.Fields().ByName("name")).String(); got != "alice" {
		t.Fatalf("name = %q", got)
	}
	if got := pm.Get(msgDesc.Fields().ByName("active")).Bool(); !got {
		t.Fatalf("active = %v", got)
	}
	if got := pm.Get(msgDesc.Fields().ByName("score")).Float(); got != 1.5 {
		t.Fatalf("score = %v", got)
	}
	if got := string(pm.Get(msgDesc.Fields().ByName("data")).Bytes()); got != "raw" {
		t.Fatalf("data = %q", got)
	}
	list := pm.Get(msgDesc.Fields().ByName("tags")).List()
	if list.Len() != 2 || list.Get(0).String() != "a" || list.Get(1).String() != "b" {
		t.Fatalf("tags = %v", list)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestDbScan2ProtoMsg_Uint64LargerThanInt64(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	columns := []string{"hash"}
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns).AddRow("9223372036854775808"))

	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}

	msgDesc := buildPhase1ScanMsgDesc(t)
	msg := dynamicpb.NewMessage(msgDesc)
	if err := DbScan2ProtoMsg(rows, msg, columns, pdbutil.BuildMsgFieldsMap(columns, msgDesc.Fields(), true)); err != nil {
		t.Fatalf("DbScan2ProtoMsg: %v", err)
	}
	if got := msg.ProtoReflect().Get(msgDesc.Fields().ByName("hash")).Uint(); got != 9223372036854775808 {
		t.Fatalf("hash = %d", got)
	}
}

func TestDbRowScanner_ReusesDestAcrossRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	columns := []string{"id", "name", "active", "score", "data", "tags"}
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns).
		AddRow(int64(1), "first", true, float64(1), []byte("a"), "[\"x\"]").
		AddRow(int64(2), nil, false, nil, nil, nil))

	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query: %v", err)
	}
	defer rows.Close()

	msgDesc := buildPhase1ScanMsgDesc(t)
	msg := dynamicpb.NewMessage(msgDesc)
	scanner, err := NewDbRowScanner(rows, msg, columns, pdbutil.BuildMsgFieldsMap(columns, msgDesc.Fields(), true))
	if err != nil {
		t.Fatalf("NewDbRowScanner: %v", err)
	}

	if !rows.Next() {
		t.Fatal("expected first row")
	}
	if err := scanner.Scan(rows, msg); err != nil {
		t.Fatalf("Scan first row: %v", err)
	}
	if got := msg.ProtoReflect().Get(msgDesc.Fields().ByName("name")).String(); got != "first" {
		t.Fatalf("first name = %q", got)
	}

	msg.Reset()
	if !rows.Next() {
		t.Fatal("expected second row")
	}
	if err := scanner.Scan(rows, msg); err != nil {
		t.Fatalf("Scan second row: %v", err)
	}
	pm := msg.ProtoReflect()
	if got := pm.Get(msgDesc.Fields().ByName("id")).Int(); got != 2 {
		t.Fatalf("second id = %d", got)
	}
	if pm.Has(msgDesc.Fields().ByName("name")) {
		t.Fatalf("second name should be unset for NULL")
	}
	if pm.Has(msgDesc.Fields().ByName("data")) {
		t.Fatalf("second data should be unset for NULL")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestDbScan2ProtoMsgx2_OddColumnCountReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	columns := []string{"id", "name", "id"}
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns).AddRow(int64(1), "old", int64(2)))

	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}

	msgDesc := buildPhase1ScanMsgDesc(t)
	oldMsg := dynamicpb.NewMessage(msgDesc)
	newMsg := dynamicpb.NewMessage(msgDesc)
	err = DbScan2ProtoMsgx2(rows, oldMsg, newMsg, columns, pdbutil.BuildMsgFieldsMap(nil, msgDesc.Fields(), true))
	if err == nil || !strings.Contains(err.Error(), "even column count") {
		t.Fatalf("expected even column count error, got %v", err)
	}
}

func TestUnwrapScanVal_NullableTypes(t *testing.T) {
	if got := unwrapScanVal(&sql.NullString{String: "x", Valid: true}); got != "x" {
		t.Fatalf("NullString = %#v", got)
	}
	if got := unwrapScanVal(&sql.NullString{String: "x", Valid: false}); got != nil {
		t.Fatalf("invalid NullString = %#v", got)
	}
	if got := unwrapScanVal(&sql.NullInt64{Int64: 3, Valid: true}); got != int64(3) {
		t.Fatalf("NullInt64 = %#v", got)
	}
	if got := unwrapScanVal(&sql.NullBool{Bool: true, Valid: true}); got != true {
		t.Fatalf("NullBool = %#v", got)
	}
	if got := unwrapScanVal(&sql.NullFloat64{Float64: 1.25, Valid: true}); got != float64(1.25) {
		t.Fatalf("NullFloat64 = %#v", got)
	}
}
