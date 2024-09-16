package pdbutil

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strings"
)

var NullValue = sql.NullString{String: "", Valid: false}

var EmptyPDB = &protodb.PDBField{}
var EmptyPDBM = &protodb.PDBMsg{}

func GetPDB(fieldDescriptor protoreflect.FieldDescriptor) (pdb *protodb.PDBField, found bool) {
	fieldOptions := fieldDescriptor.Options()
	if fieldOptions == nil {
		return EmptyPDB, false
	}

	if !proto.HasExtension(fieldOptions, protodb.E_Pdb) {
		return EmptyPDB, false
	}

	return proto.GetExtension(fieldOptions, protodb.E_Pdb).(*protodb.PDBField), true
}

func GetPDBM(msgDescriptor protoreflect.MessageDescriptor) (pdbm *protodb.PDBMsg, found bool) {
	msgOptions := msgDescriptor.Options()
	if msgOptions == nil {
		return EmptyPDBM, false
	}

	if !proto.HasExtension(msgOptions, protodb.E_Pdbm) {
		return EmptyPDBM, false
	}

	return proto.GetExtension(msgOptions, protodb.E_Pdbm).(*protodb.PDBMsg), true
}

func IsZeroValue(val interface{}) bool {
	valStr := fmt.Sprint(val)
	return len(valStr) == 0 || valStr == "0"
}

func MaybeNull(val interface{}, field protoreflect.FieldDescriptor, fieldpdb *protodb.PDBField) interface{} {
	if IsZeroValue(val) {
		return NullValue
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
		fieldPdb, _ := GetPDB(field)
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

// GetPrimaryKeyOrUniqueFieldDescs get primary key or unique field descriptors, primaryKey|unique -> field descriptor
func GetPrimaryKeyOrUniqueFieldDescs(msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, nameLowercase bool) map[string]protoreflect.FieldDescriptor {
	result := make(map[string]protoreflect.FieldDescriptor)

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)
		fieldPdb, _ := GetPDB(field)
		if fieldPdb != nil && (fieldPdb.IsPrimary() || fieldPdb.IsUnique() || len(fieldPdb.UniqueName) > 0) {
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
