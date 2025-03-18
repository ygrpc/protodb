package crud

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb/msgstore"
	"strings"

	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type TqueryItem struct {
	Err   *string
	IsEnd bool
	Msg   proto.Message
}

// DbTableQuery executes a query against the database and streams results through the provided channel
func DbTableQuery(db *sql.DB, msg proto.Message, where map[string]string, resultColumns []string, schemaName string, tableName string, permissionSqlStr string, resultCh chan TqueryItem) (err error) {

	if len(resultColumns) > 0 {
		err = checkSQLColumnsIsNoInjection(resultColumns)
		if err != nil {
			return err
		}
	}
	go dbTableQueryRoutine(db, msg, where, resultColumns, schemaName, tableName, permissionSqlStr, resultCh)
	return nil
}
func dbTableQueryRoutine(db *sql.DB, msg proto.Message, where map[string]string, resultColumns []string, schemaName string, tableName string, permissionSqlStr string, resultCh chan TqueryItem) {
	defer close(resultCh)

	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlTableQuery(msg, where, resultColumns, schemaName, tableName, msgDesc, msgFieldDescs, dbdialect, permissionSqlStr)
	if err != nil {
		errStr := err.Error()
		resultCh <- TqueryItem{
			Err:   &errStr,
			IsEnd: true,
		}
		return
	}

	rows, err := db.Query(sqlStr, sqlVals...)
	if err != nil {
		errStr := err.Error()
		resultCh <- TqueryItem{
			Err:   &errStr,
			IsEnd: true,
		}
		return
	}
	defer rows.Close()

	// Prepare field map for scanning
	var msgFieldsMap map[string]protoreflect.FieldDescriptor
	if len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*") {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(nil, msgDesc.Fields(), true)
	} else {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(resultColumns, msgDesc.Fields(), true)
	}

	var rowMsg proto.Message

	//isEnd := false
	ok := false

	if rows.Next() {
		rowMsg, ok = msgstore.GetMsg(tableName, true)
		if ok != true {
			errStr := fmt.Errorf("can not get protodb msg %s err", tableName).Error()
			resultCh <- TqueryItem{
				Err:   &errStr,
				IsEnd: true,
			}
			return
		}

		// Scan the row into the message
		if len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*") {
			err = DbScan2ProtoMsg(rows, rowMsg, nil, msgFieldsMap)
		} else {
			err = DbScan2ProtoMsg(rows, rowMsg, resultColumns, msgFieldsMap)
		}

		if err != nil {
			errStr := err.Error()
			resultCh <- TqueryItem{
				Err:   &errStr,
				IsEnd: true,
			}
			return
		}
	}
	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		errStr := err.Error()
		resultCh <- TqueryItem{
			Err:   &errStr,
			IsEnd: true,
		}
		return
	}

	// Process each row
	for rows.Next() {
		// Send previous message
		resultCh <- TqueryItem{
			Msg:   rowMsg,
			IsEnd: false,
		}

		// Create a new message instance for each row
		rowMsg, _ = msgstore.GetMsg(tableName, true)

		// Scan the row into the message
		if len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*") {
			err = DbScan2ProtoMsg(rows, rowMsg, nil, msgFieldsMap)
		} else {
			err = DbScan2ProtoMsg(rows, rowMsg, resultColumns, msgFieldsMap)
		}

		if err != nil {
			errStr := err.Error()
			resultCh <- TqueryItem{
				Err:   &errStr,
				IsEnd: true,
			}
			return
		}

	}

	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		errStr := err.Error()
		resultCh <- TqueryItem{
			Err:   &errStr,
			IsEnd: true,
		}
		return
	}

	// Send final item indicating end of results
	resultCh <- TqueryItem{
		IsEnd: true,
		Msg:   rowMsg,
	}
	return

}

// dbBuildSqlTableQuery builds a SQL query for table query
func dbBuildSqlTableQuery(msg proto.Message, where map[string]string, resultColumns []string, schemaName string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect,
	permissionSqlStr string) (sqlStr string, sqlVals []interface{}, err error) {

	sb := strings.Builder{}
	sb.WriteString(protosql.SQL_SELECT)

	if len(resultColumns) == 0 {
		sb.WriteString(protosql.SQL_ASTERISK)
	} else {
		sb.WriteString(strings.Join(resultColumns, protosql.SQL_COMMA))
	}

	sb.WriteString(protosql.SQL_FROM)

	dbtableName := sqldb.BuildDbTableName(tableName, schemaName, dbdialect)
	sb.WriteString(dbtableName)

	// Add WHERE clauses if any
	//hasWhere := false
	placeholder := dbdialect.Placeholder()
	sqlParaNo := 1

	if len(where) > 0 || len(permissionSqlStr) > 0 {
		sb.WriteString(protosql.SQL_WHERE)
		//hasWhere = true

		firstPlaceholder := true

		// Add permission SQL if provided
		if len(permissionSqlStr) > 0 {
			sb.WriteString(permissionSqlStr)
			firstPlaceholder = false
		}

		// Add conditions from where map
		for fieldName, fieldValue := range where {
			if !firstPlaceholder {
				sb.WriteString(protosql.SQL_AND)
			}
			firstPlaceholder = false

			sb.WriteString(fieldName)
			sb.WriteString(protosql.SQL_EQUEAL)

			if placeholder == protosql.SQL_QUESTION {
				sb.WriteString(string(protosql.SQL_QUESTION))
			} else {
				sb.WriteString(string(protosql.SQL_DOLLAR))
				sb.WriteString(fmt.Sprint(sqlParaNo))
				sqlParaNo++
			}

			sqlVals = append(sqlVals, fieldValue)
		}
	}

	sqlStr = sb.String()

	return sqlStr, sqlVals, nil
}
