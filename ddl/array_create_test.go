package ddl

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }

func buildDDLArrayDescriptors(t *testing.T) (msg proto.Message, msgDesc protoreflect.MessageDescriptor, idField protoreflect.FieldDescriptor, numsField protoreflect.FieldDescriptor, subsField protoreflect.FieldDescriptor) {
	t.Helper()

	idOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(idOpts, protodb.E_Pdb, &protodb.PDBField{Primary: true})

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("ddl_array_test.proto"),
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
						Name:    strPtr("id"),
						Number:  int32Ptr(1),
						Label:   descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:    descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
						Options: idOpts,
					},
					{
						Name:   strPtr("nums"),
						Number: int32Ptr(2),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
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

	msgDesc = fd.Messages().ByName("TestMsg")
	idField = msgDesc.Fields().ByName("id")
	numsField = msgDesc.Fields().ByName("nums")
	subsField = msgDesc.Fields().ByName("subs")
	msg = dynamicpb.NewMessage(msgDesc)
	return msg, msgDesc, idField, numsField, subsField
}

func TestGetSqlTypeStr_RepeatedFields(t *testing.T) {
	_, _, _, numsField, subsField := buildDDLArrayDescriptors(t)

	got := getSqlTypeStr(numsField, &protodb.PDBField{DbTypeStr: "text", DbType: protodb.FieldDbType_TEXT}, sqldb.Postgres)
	if got != "text" {
		t.Fatalf("expected text (user DbType/DbTypeStr takes precedence), got %q", got)
	}

	got = getSqlTypeStr(subsField, &protodb.PDBField{}, sqldb.Postgres)
	if got != "jsonb" {
		t.Fatalf("expected jsonb, got %q", got)
	}

	got = getSqlTypeStr(numsField, &protodb.PDBField{}, sqldb.SQLite)
	if got != "text" {
		t.Fatalf("expected text, got %q", got)
	}
}

func TestDbCreateSQL_RepeatedDefaults_Postgres(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	msg, _, _, _, _ := buildDDLArrayDescriptors(t)
	initMap := map[string]*TDbTableInitSql{}
	item, err := DbCreateSQL(db, msg, "", false, false, initMap)
	if err != nil {
		t.Fatalf("DbCreateSQL: %v", err)
	}
	if len(item.SqlStr) != 1 {
		t.Fatalf("expected 1 sql, got %d", len(item.SqlStr))
	}
	sqlStr := item.SqlStr[0]
	if !strings.Contains(sqlStr, "nums bigint[]") || !strings.Contains(sqlStr, "DEFAULT '{}'::bigint[]") {
		t.Fatalf("missing nums default: %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "subs jsonb") || !strings.Contains(sqlStr, "DEFAULT '[]'::jsonb") {
		t.Fatalf("missing subs default: %s", sqlStr)
	}
}
