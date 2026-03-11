package crud

import (
	"database/sql"
	"fmt"

	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func mysqlSupportsReturning(dialect sqldb.TDBDialect) bool {
	return dialect != sqldb.Mysql
}

func mysqlSelectReturnedMsg(db sqldb.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors) (proto.Message, error) {
	return dbSelectOne(db, msg, nil, nil, dbschema, tableName, msgDesc, msgFieldDescs, sqldb.Mysql, true)
}

func mysqlPopulateInsertPrimaryKey(msg proto.Message, msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, result sql.Result) error {
	primaryKeyFields := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)
	if len(primaryKeyFields) == 0 {
		return fmt.Errorf("no primary key field")
	}

	pm := msg.ProtoReflect()
	allKeysPresent := true
	var missingField protoreflect.FieldDescriptor
	for _, field := range primaryKeyFields {
		if isZeroPrimaryKeyValue(pm.Get(field), field) {
			allKeysPresent = false
			if missingField != nil {
				return fmt.Errorf("mysql insert return requires a single retrievable primary key")
			}
			missingField = field
		}
	}
	if allKeysPresent {
		return nil
	}
	if missingField == nil {
		return fmt.Errorf("mysql insert return cannot determine primary key")
	}

	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("mysql insert return requires LastInsertId: %w", err)
	}
	switch missingField.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		pm.Set(missingField, protoreflect.ValueOfInt32(int32(lastInsertID)))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		pm.Set(missingField, protoreflect.ValueOfInt64(lastInsertID))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		if lastInsertID < 0 {
			return fmt.Errorf("mysql insert returned negative id %d for unsigned primary key", lastInsertID)
		}
		pm.Set(missingField, protoreflect.ValueOfUint32(uint32(lastInsertID)))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if lastInsertID < 0 {
			return fmt.Errorf("mysql insert returned negative id %d for unsigned primary key", lastInsertID)
		}
		pm.Set(missingField, protoreflect.ValueOfUint64(uint64(lastInsertID)))
	default:
		return fmt.Errorf("mysql insert return unsupported primary key kind %v", missingField.Kind())
	}

	return nil
}

func isZeroPrimaryKeyValue(val protoreflect.Value, field protoreflect.FieldDescriptor) bool {
	switch field.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return val.Int() == 0
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return val.Int() == 0
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return val.Uint() == 0
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return val.Uint() == 0
	case protoreflect.StringKind:
		return val.String() == ""
	case protoreflect.BoolKind:
		return !val.Bool()
	case protoreflect.EnumKind:
		return val.Enum() == 0
	case protoreflect.BytesKind:
		return len(val.Bytes()) == 0
	default:
		return !val.IsValid()
	}
}
