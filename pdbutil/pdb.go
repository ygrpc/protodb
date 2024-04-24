package pdbutil

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strings"
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

func GetPDBM(msgDescriptor protoreflect.MessageDescriptor) (pdbm *protodb.PDBMsg) {
	msgOptions := msgDescriptor.Options()
	if msgOptions == nil {
		return
	}

	if !proto.HasExtension(msgOptions, protodb.E_Pdbm) {
		return
	}

	return proto.GetExtension(msgOptions, protodb.E_Pdbm).(*protodb.PDBMsg)
}

func MaybeNull(val interface{}, field protoreflect.FieldDescriptor, fieldpdb *protodb.PDBField) interface{} {
	valStr := fmt.Sprint(val)
	if len(valStr) == 0 || valStr == "0" {
		return sql.NullString{String: "", Valid: false}
	}
	return val
}

// BuildMsgFieldsMap build msgFieldsMap, if columnNames is nil, return all msg fields
func BuildMsgFieldsMap(fieldNames []string, msgFieldsDesc protoreflect.FieldDescriptors, nameLowercase bool) map[string]protoreflect.FieldDescriptor {
	columnNamesMap := make(map[string]bool)
	for _, columnName := range fieldNames {
		columnNamesMap[strings.ToLower(columnName)] = true
	}

	msgFieldsMap := make(map[string]protoreflect.FieldDescriptor)

	for i := 0; i < msgFieldsDesc.Len(); i++ {
		fieldDesc := msgFieldsDesc.Get(i)
		fieldName := string(fieldDesc.Name())
		fieldNameLowercase := strings.ToLower(fieldName)
		if _, ok := columnNamesMap[fieldNameLowercase]; ok || fieldNames == nil {
			if nameLowercase {
				msgFieldsMap[fieldNameLowercase] = fieldDesc
			} else {
				msgFieldsMap[fieldName] = fieldDesc
			}
		}
	}

	return msgFieldsMap
}

// GetPrimaryKeyFieldDescs get primary key field descriptors, primaryKey(lowercase) -> field descriptor
func GetPrimaryKeyFieldDescs(msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, nameLowercase bool) map[string]protoreflect.FieldDescriptor {
	result := make(map[string]protoreflect.FieldDescriptor)

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)
		fieldPdb := GetPDB(field)
		if fieldPdb != nil && fieldPdb.IsPrimary() {
			fieldName := string(field.Name())
			if nameLowercase {
				result[strings.ToLower(fieldName)] = field
			} else {
				result[fieldName] = field
			}
		}
	}

	return result
}
