package pdbutil

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func GetPDB(fieldDescriptor protoreflect.FieldDescriptor) (pdb *protodb.PDBField) {
	fieldOptions := fieldDescriptor.Options()
	if fieldOptions == nil {
		return
	}

	if !proto.HasExtension(fieldOptions, protodb.E_Pdb) {
		return
	}

	return proto.GetExtension(fieldOptions, protodb.E_Pdb).(*protodb.PDBField)
}

func GetPDBM(msgDescriptor protoreflect.MessageDescriptor) (pdb *protodb.PDBField) {
	msgOptions := msgDescriptor.Options()
	if msgOptions == nil {
		return
	}

	if !proto.HasExtension(msgOptions, protodb.E_Pdbm) {
		return
	}

	return proto.GetExtension(msgOptions, protodb.E_Pdbm).(*protodb.PDBField)
}

func MaybeNull(val interface{}, field protoreflect.FieldDescriptor, fieldpdb *protodb.PDBField) interface{} {
	valStr := fmt.Sprint(val)
	if len(valStr) == 0 || valStr == "0" {
		return sql.NullString{String: "", Valid: false}
	}
	return val
}
