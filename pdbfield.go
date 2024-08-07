package protodb

import (
	"fmt"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// not in db
func (x *PDBField) IsNotDB() bool {
	return x.NotDB
}

// is primary key
func (x *PDBField) IsPrimary() bool {
	return x.Primary
}

// is auto increment
func (x *PDBField) IsAutoIncrement() bool {
	return x.SerialType != 0
}

// is serial type == isautoincrement
func (x *PDBField) IsSerial() bool {
	return x.SerialType != 0
}

// is unique
func (x *PDBField) IsUnique() bool {
	return x.Unique
}

// is not null
func (x *PDBField) IsNotNull() bool {
	return x.NotNull
}

// is reference
func (x *PDBField) IsReference() bool {
	return len(x.Reference) > 0
}

// is no update
func (x *PDBField) IsNoUpdate() bool {
	return x.NoUpdate
}

// zero as null
func (x *PDBField) IsZeroAsNull() bool {
	return x.ZeroAsNull
}

// need in insert
func (x *PDBField) NeedInInsert() bool {
	if x.NotDB {
		return false
	}
	if x.NoInsert {
		return false
	}

	if x.IsAutoIncrement() {
		return false
	}

	return true
}

// need in update
func (x *PDBField) NeedInUpdate() bool {
	if x.NotDB {
		return false
	}

	if x.NoUpdate {
		return false
	}

	if x.Primary {
		return false
	}

	return true
}

// has default value
func (x *PDBField) HasDefaultValue() (defaultValue string, found bool) {
	return x.DefaultValue, len(x.DefaultValue) > 0
}

// default value to sql args
func (x *PDBField) DefaultValue2SQLArgs() (sqlArgs interface{}) {
	//todo need more logic to convert default value to sql args
	return x.DefaultValue
}

// PdbDbTypeStr get db type string from pdb if specified
func (x *PDBField) PdbDbTypeStr(fieldMsg protoreflect.FieldDescriptor) string {
	if len(x.DbTypeStr) > 0 {
		return x.DbTypeStr
	}

	switch x.DbType {
	case FieldDbType_AutoMatch:
		switch x.SerialType {
		case 0:
			return GetProtoDBType(fieldMsg.Kind())
		case 2:
			return "smallserial"
		case 4:
			return "serial"
		case 8:
			return "bigserial"

		default:
			fmt.Println("todo: PdbDbTypeStr unknown serial type:", x.SerialType)
			return "text"
		}

	//bool
	case FieldDbType_BOOL:
		return "boolean"
		//int32
	case FieldDbType_INT32:
		return "integer"
		//uint32
	case FieldDbType_UINT32:
		return "bigint"
		//int64
	case FieldDbType_INT64:
		return "bigint"
		//float
	case FieldDbType_FLOAT:
		return "real"
	case FieldDbType_DOUBLE:
		return "double precision"
		//text
	case FieldDbType_TEXT:
		return "text"
		//jsonb
	case FieldDbType_JSONB:
		return "jsonb"
		//uuid
	case FieldDbType_UUID:
		return "uuid"
		//timestamp
	case FieldDbType_TIMESTAMP:
		return "timestamp"
		//date
	case FieldDbType_DATE:
		return "date"
		//bytea
	case FieldDbType_BYTEA:
		return "bytea"
		//inet
	case FieldDbType_INET:
		return "inet"

	default:
		fmt.Println("todo: PdbDbTypeStr unknown db type:", x.DbType)
		return ""
	}

}

func GetProtoDBType(fieldType protoreflect.Kind) string {
	switch fieldType {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "integer"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "bigint"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "bigint"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "bigint"
	case protoreflect.FloatKind:
		return "real"
	case protoreflect.DoubleKind:
		return "double precision"
	case protoreflect.StringKind:
		return "text"
	case protoreflect.BytesKind:
		return "bytea"
	case protoreflect.EnumKind:
		return "integer"
	case protoreflect.MessageKind:
		return "jsonb"

	default:
		return "text"
	}
}
