package crud

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strings"
)

// interface FieldProtoMsg
type FieldProtoMsg interface {
	GetFieldProtoMsg(fieldName string) (proto.Message, bool)
}

// DbScan2ProtoMsg scan db rows to proto message, the proto msg has no nested message
func DbScan2ProtoMsg(rows *sql.Rows, msg proto.Message, columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) (err error) {
	if columnNames == nil {
		columnNames, err = rows.Columns()
		if err != nil {
			return err
		}
	}

	rowVals := make([]interface{}, len(columnNames))
	for i := range rowVals {
		rowVals[i] = new(interface{})
	}
	err = rows.Scan(rowVals...)
	if err != nil {
		fmt.Println("DbScan2ProtoMsg err:", err)
		return err
	}

	if msgFieldsMap == nil {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(columnNames, msg.ProtoReflect().Descriptor().Fields(), true)
	}

	for i := 0; i < len(columnNames); i++ {
		columnName := strings.ToLower(columnNames[i])
		fieldDesc, ok := msgFieldsMap[columnName]
		if !ok {
			fmt.Println("DbScan2ProtoMsg field not found in msgFieldsMap :", columnName)
			fmt.Println("msgFieldsMap:", msgFieldsMap)
			continue

		}

		err = SetProtoMsgField(msg, fieldDesc, rowVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsg SetProtoMsgField err:", err)
			return err

		}
	}

	return nil
}

func Val2ProtoMsgByJson(msg proto.Message, value interface{}) error {
	if value == nil {
		return nil
	}

	var b []byte // This will hold the JSON bytes

	// 3. Use a type switch to determine the type of 'value' and get bytes
	switch v := value.(type) {
	case string:
		// If it's a string, convert it to a byte slice
		b = []byte(v)
	case []byte:
		// If it's already a byte slice, use it directly
		b = v
	case *string:
		// If it's a pointer to a string
		if v == nil {
			return nil
		}
		b = []byte(*v) // Dereference and convert
	case *[]byte:
		// If it's a pointer to a byte slice
		if v == nil {
			return nil
		}
		b = *v // Dereference
	default:
		// If it's none of the expected types
		return fmt.Errorf("unsupported type for value: %T", value)
	}

	// 4. Unmarshal the JSON bytes into the proto message
	// You can use default options or configure them:
	unmarshalOpts := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Example: Ignore fields in JSON not in the proto
	}
	// return protojson.Unmarshal(b, msg) // Using default options
	return unmarshalOpts.Unmarshal(b, msg) // Using configured options
}

func SetProtoMsgField(msg proto.Message, fieldDesc protoreflect.FieldDescriptor, fieldVal interface{}) error {
	fieldName := fieldDesc.TextName()
	if fieldDesc.Kind() == protoreflect.MessageKind {
		// get filed proto msg by filedprotomsg interface
		filedProtoMsg, ok := msg.(FieldProtoMsg)
		if ok {
			fieldMsg, fieldMsgOk := filedProtoMsg.GetFieldProtoMsg(fieldName)
			if !fieldMsgOk {
				fieldMsgProto := fieldDesc.Message()
				return fmt.Errorf("GetFieldProtoMsg:can't get filed proto msg for field %s.%s", fieldMsgProto.Name(), fieldName)
			}
			err := Val2ProtoMsgByJson(fieldMsg, fieldVal)
			if err != nil {
				fieldMsgProto := fieldDesc.Message()
				return fmt.Errorf("Val2ProtoMsgByJson err:%s for field %s.%s val:%v", err.Error(), fieldMsgProto.Name(), fieldName, fieldVal)
			}
			return pdbutil.SetField(msg, fieldName, fieldMsg)
		}

		//get filed proto msg by filedmsgstore
		fieldMsgProto := fieldDesc.Message()
		fieldMsgName := string(fieldMsgProto.Name())
		fieldMsg, fieldMsgOk := msgstore.GetFieldMsg(fieldMsgName, true)
		if !fieldMsgOk {
			return fmt.Errorf("can't get filed proto msg for field %s.%s, you can register using msgstore.RegisterFieldMsg", fieldMsgProto.Name(), fieldName)
		}
		err := Val2ProtoMsgByJson(fieldMsg, fieldVal)
		if err != nil {
			return fmt.Errorf("Val2ProtoMsgByJson err:%s for field %s.%s val:%v", err.Error(), fieldMsgProto.Name(), fieldName, fieldVal)
		}
		return pdbutil.SetField(msg, fieldName, fieldMsg)

	}
	return pdbutil.SetField(msg, fieldName, fieldVal)
}

// DbScan2ProtoMsgx2 scan db rows to proto message(oldMsg and newMsg), the proto msg has no nested message
func DbScan2ProtoMsgx2(rows *sql.Rows, oldMsg proto.Message, newMsg proto.Message, columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) (err error) {
	if columnNames == nil {
		columnNames, err = rows.Columns()
		if err != nil {
			return err
		}
	}

	rowVals := make([]interface{}, len(columnNames))
	for i := range rowVals {
		rowVals[i] = new(interface{})
	}
	err = rows.Scan(rowVals...)
	if err != nil {
		fmt.Println("DbScan2ProtoMsgx2 err:", err)
		return err
	}

	columnNamesOld := columnNames[:len(columnNames)/2]
	// columnNamesNew:=columnNames[len(columnNames)/2:]

	oldVals := rowVals[:len(rowVals)/2]
	newVals := rowVals[len(rowVals)/2:]

	if msgFieldsMap == nil {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(columnNames, oldMsg.ProtoReflect().Descriptor().Fields(), true)
	}

	for i := 0; i < len(columnNamesOld); i++ {
		columnName := strings.ToLower(columnNamesOld[i])
		fieldDesc, ok := msgFieldsMap[columnName]
		if !ok {
			fmt.Println("DbScan2ProtoMsgx2 field not found in msgFieldsMap :", columnName)
			fmt.Println("msgFieldsMap:", msgFieldsMap)
			continue

		}

		err = SetProtoMsgField(oldMsg, fieldDesc, oldVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsgx2 SetProtoMsgField err:", err)
			return err

		}

		err = SetProtoMsgField(newMsg, fieldDesc, newVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsgx2 SetProtoMsgField err:", err)
			return err

		}
	}

	return nil
}
