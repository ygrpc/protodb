package crud

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strings"
)

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

func SetProtoMsgField(msg proto.Message, fieldDesc protoreflect.FieldDescriptor, fieldVal interface{}) error {
	return pdbutil.SetField(msg, fieldDesc.TextName(), fieldVal)
}
