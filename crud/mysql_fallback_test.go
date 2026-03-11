package crud

import (
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

func buildCRUDMySQLMessageDesc(t *testing.T) (protoreflect.MessageDescriptor, protoreflect.FieldDescriptor, protoreflect.FieldDescriptor, protoreflect.FieldDescriptor) {
	t.Helper()

	idOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(idOpts, protodb.E_Pdb, &protodb.PDBField{Primary: true, NoInsert: true})

	fdp := &descriptorpb.FileDescriptorProto{
		Syntax:  strPtr("proto3"),
		Name:    strPtr("crud_mysql_test.proto"),
		Package: strPtr("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("User"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:    strPtr("id"),
						Number:  int32Ptr(1),
						Label:   descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:    descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
						Options: idOpts,
					},
					{
						Name:   strPtr("name"),
						Number: int32Ptr(2),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
					{
						Name:   strPtr("tags"),
						Number: int32Ptr(3),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
		},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}

	msgDesc := fd.Messages().ByName("User")
	return msgDesc, msgDesc.Fields().ByName("id"), msgDesc.Fields().ByName("name"), msgDesc.Fields().ByName("tags")
}

func newUserMessage(t *testing.T, id int64, name string, tags []string) (proto.Message, protoreflect.MessageDescriptor) {
	t.Helper()

	msgDesc, idField, nameField, tagsField := buildCRUDMySQLMessageDesc(t)
	msg := dynamicpb.NewMessage(msgDesc)
	if id != 0 {
		msg.ProtoReflect().Set(idField, protoreflect.ValueOfInt64(id))
	}
	if name != "" {
		msg.ProtoReflect().Set(nameField, protoreflect.ValueOfString(name))
	}
	if tags != nil {
		list := msg.ProtoReflect().Mutable(tagsField).List()
		for _, tag := range tags {
			list.Append(protoreflect.ValueOfString(tag))
		}
	}
	return msg, msgDesc
}

func TestEncodeSQLArg_RepeatedString_MysqlJSON(t *testing.T) {
	_, _, _, tagsField := buildCRUDMySQLMessageDesc(t)
	v, err := EncodeSQLArg(tagsField, sqldb.Mysql, []string{"a", "b"})
	if err != nil {
		t.Fatalf("EncodeSQLArg: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	if s != "[\"a\",\"b\"]" {
		t.Fatalf("unexpected json: %s", s)
	}
}

func TestMysqlSupportsReturning(t *testing.T) {
	if mysqlSupportsReturning(sqldb.Mysql) {
		t.Fatalf("mysql should not use RETURNING")
	}
	if !mysqlSupportsReturning(sqldb.Postgres) {
		t.Fatalf("postgres should support RETURNING")
	}
}

func TestMysqlPopulateInsertPrimaryKey_FromLastInsertID(t *testing.T) {
	msg, msgDesc := newUserMessage(t, 0, "alice", []string{"a", "b"})
	idField := msgDesc.Fields().ByName("id")

	if err := mysqlPopulateInsertPrimaryKey(msg, msgDesc, msgDesc.Fields(), sqlmock.NewResult(7, 1)); err != nil {
		t.Fatalf("mysqlPopulateInsertPrimaryKey: %v", err)
	}
	if got := msg.ProtoReflect().Get(idField).Int(); got != 7 {
		t.Fatalf("unexpected id: %d", got)
	}
}

func TestMysqlPopulateInsertPrimaryKey_KeepsExistingKey(t *testing.T) {
	msg, msgDesc := newUserMessage(t, 9, "alice", nil)
	idField := msgDesc.Fields().ByName("id")

	if err := mysqlPopulateInsertPrimaryKey(msg, msgDesc, msgDesc.Fields(), sqlmock.NewResult(7, 1)); err != nil {
		t.Fatalf("mysqlPopulateInsertPrimaryKey: %v", err)
	}
	if got := msg.ProtoReflect().Get(idField).Int(); got != 9 {
		t.Fatalf("unexpected id overwrite: %d", got)
	}
}
