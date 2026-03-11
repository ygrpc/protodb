package ddl

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func compactSQL(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func buildDDLMySQLDescriptors(t *testing.T) (proto.Message, protoreflect.MessageDescriptor) {
	t.Helper()

	idOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(idOpts, protodb.E_Pdb, &protodb.PDBField{Primary: true})

	emailOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(emailOpts, protodb.E_Pdb, &protodb.PDBField{Unique: true, NotNull: true})

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("ddl_mysql_test.proto"),
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
						Name:    strPtr("email"),
						Number:  int32Ptr(2),
						Label:   descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:    descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Options: emailOpts,
					},
					{
						Name:   strPtr("tags"),
						Number: int32Ptr(3),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
					{
						Name:     strPtr("subs"),
						Number:   int32Ptr(4),
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

	msgDesc := fd.Messages().ByName("TestMsg")
	return dynamicpb.NewMessage(msgDesc), msgDesc
}

func TestCreateOneUniqueKeySql_MysqlOmitsIfNotExists(t *testing.T) {
	msg, msgDesc := buildDDLMySQLDescriptors(t)
	_ = msg

	emailField := msgDesc.Fields().ByName("email")
	sqlStr := createOneUniqueKeySql("TestMsg", "uk_TestMsg_email", []protoreflect.FieldDescriptor{emailField}, sqldb.Mysql)
	if strings.Contains(sqlStr, "IF NOT EXISTS") {
		t.Fatalf("mysql unique index sql should not contain IF NOT EXISTS: %s", sqlStr)
	}
	if compactSQL(sqlStr) != "CREATE UNIQUE INDEX uk_TestMsg_email ON TestMsg ( email ) ;" {
		t.Fatalf("unexpected mysql unique index sql: %s", sqlStr)
	}
}

func TestDbMigrateTableMysql_AddsMissingColumnsAndUniqueIndex(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	msg, msgDesc := buildDDLMySQLDescriptors(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS (SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?)")).
		WithArgs("TestMsg").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?")).
		WithArgs("TestMsg").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).AddRow("id"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT column_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ? ORDER BY seq_in_index")).
		WithArgs("TestMsg", "uk_TestMsg_email").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))

	migrateItem := &TDbTableInitSql{
		TableName:          "TestMsg",
		SqlStr:             make([]string, 0),
		DepTableNames:      make([]string, 0),
		DepTableSqlItemMap: make(map[string]*TDbTableInitSql),
	}
	got, err := dbMigrateTableMysql(migrateItem, db, msg, "", "TestMsg", msgDesc, msgDesc.Fields(), false, false, map[string]*TDbTableInitSql{})
	if err != nil {
		t.Fatalf("dbMigrateTableMysql: %v", err)
	}
	if len(got.SqlStr) != 4 {
		t.Fatalf("expected 4 sql fragments, got %#v", got.SqlStr)
	}

	all := strings.Join(got.SqlStr, "\n")
	if !strings.Contains(compactSQL(all), "ALTER TABLE TestMsg ADD COLUMN email text NOT NULL ;") {
		t.Fatalf("missing email alter sql: %s", all)
	}
	if !strings.Contains(compactSQL(all), "ALTER TABLE TestMsg ADD COLUMN tags json NOT NULL DEFAULT (CAST('[]' AS JSON));") {
		t.Fatalf("missing tags alter sql: %s", all)
	}
	if !strings.Contains(compactSQL(all), "ALTER TABLE TestMsg ADD COLUMN subs json NOT NULL DEFAULT (CAST('[]' AS JSON));") {
		t.Fatalf("missing subs alter sql: %s", all)
	}
	if !strings.Contains(compactSQL(all), "CREATE UNIQUE INDEX uk_TestMsg_email ON TestMsg ( email ) ;") {
		t.Fatalf("missing unique index sql: %s", all)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
