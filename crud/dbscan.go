package crud

import (
	"database/sql"
	"fmt"
	"google.golang.org/protobuf/proto"
)

func DbScan2ProtoMsg(rows *sql.Rows, msg proto.Message, columnNames []string) (err error) {
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

	return nil
}
