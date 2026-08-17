package crud

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
)

type insertTestMessage struct {
	FirstField    string
	NullableField string
	reflection    protoreflect.Message
}

func (m *insertTestMessage) ProtoReflect() protoreflect.Message {
	return m.reflection
}

func TestDbBuildSqlInsert_TruncatedNullableZeroFieldWritesNull(t *testing.T) {
	tests := []struct {
		name string
		pdb  *protodb.PDBField
	}{
		{name: "zero as null", pdb: &protodb.PDBField{ZeroAsNull: true}},
		{name: "reference", pdb: &protodb.PDBField{Reference: "Parent(Id)"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := newInsertTestMessage(t, tt.pdb)
			desc := msg.ProtoReflect().Descriptor()

			sqlStr, vals, err := dbBuildSqlInsert(msg, 1, "", "", desc, desc.Fields(), sqldb.Postgres, false)
			if err != nil {
				t.Fatalf("dbBuildSqlInsert: %v", err)
			}
			if !strings.Contains(sqlStr, "NullableField") {
				t.Fatalf("truncated nullable field missing from SQL: %s", sqlStr)
			}
			if len(vals) != 2 {
				t.Fatalf("vals length = %d, want 2: %#v", len(vals), vals)
			}
			nullValue, ok := vals[1].(sql.NullString)
			if !ok || nullValue.Valid {
				t.Fatalf("truncated nullable field value = %#v, want invalid sql.NullString", vals[1])
			}
		})
	}
}

func TestDbBuildSqlInsert_TruncatedNotNullZeroAsNullFieldStillErrors(t *testing.T) {
	msg := newInsertTestMessage(t, &protodb.PDBField{ZeroAsNull: true, NotNull: true})
	desc := msg.ProtoReflect().Descriptor()

	_, _, err := dbBuildSqlInsert(msg, 1, "", "", desc, desc.Fields(), sqldb.Postgres, false)
	if err == nil || !strings.Contains(err.Error(), "NullableField is not set and has no default value") {
		t.Fatalf("dbBuildSqlInsert error = %v, want missing default value error", err)
	}
}

func newInsertTestMessage(t *testing.T, fieldPDB *protodb.PDBField) *insertTestMessage {
	t.Helper()

	fieldOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(fieldOpts, protodb.E_Pdb, fieldPDB)
	fileProto := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("crud_insert_test.proto"),
		Package: proto.String("crudtest"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("InsertMessage"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:   proto.String("FirstField"),
					Number: proto.Int32(1),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:    proto.String("NullableField"),
					Number:  proto.Int32(2),
					Label:   descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:    descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Options: fieldOpts,
				},
			},
		}},
	}

	fileDesc, err := protodesc.NewFile(fileProto, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	reflection := dynamicpb.NewMessage(fileDesc.Messages().ByName("InsertMessage"))
	return &insertTestMessage{reflection: reflection}
}
