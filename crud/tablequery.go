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
func dbTableQueryRoutine(db *sql.DB, msg proto.Message, where map[string]string, resultColumns []string,
	schemaName string, tableName string, permissionSqlStr string, resultCh chan TqueryItem) {
	defer close(resultCh)

	// Helper function to send error and return
	sendError := func(err error) {
		errStr := err.Error()
		resultCh <- TqueryItem{
			Err:   &errStr,
			IsEnd: true,
		}
	}

	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	dbdialect := sqldb.GetDBDialect(db)

	// Build SQL query
	sqlStr, sqlVals, err := dbBuildSqlTableQuery(where, resultColumns, schemaName, tableName, dbdialect, permissionSqlStr)
	if err != nil {
		sendError(err)
		return
	}

	// Execute query
	rows, err := db.Query(sqlStr, sqlVals...)
	if err != nil {
		sendError(err)
		return
	}
	defer rows.Close()

	// Determine which fields to scan
	useAllFields := len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*")
	fieldNames := resultColumns
	if useAllFields {
		fieldNames = nil
	}

	msgFieldsMap := pdbutil.BuildMsgFieldsMap(fieldNames, msgDesc.Fields(), true)

	msg = nil

	// Process rows
	isFirstRow := true
	for rows.Next() {
		// Create new message instance
		rowMsg, ok := msgstore.GetMsg(tableName, true)
		if !ok {
			sendError(fmt.Errorf("cannot get protodb msg %s", tableName))
			return
		}

		// Scan row data
		err = DbScan2ProtoMsg(rows, rowMsg,
			fieldNames,
			msgFieldsMap,
		)
		if err != nil {
			sendError(err)
			return
		}

		// Send previous row (except for first iteration)
		if !isFirstRow {
			resultCh <- TqueryItem{
				Msg:   msg,
				IsEnd: false,
			}
		}
		isFirstRow = false

		// Save current message for next iteration or final send
		msg = rowMsg
	}

	// Check for errors from row iteration
	if err = rows.Err(); err != nil {
		sendError(err)
		return
	}

	// Send final result
	resultCh <- TqueryItem{
		Msg:   msg,
		IsEnd: true,
	}
}

// dbBuildSqlTableQuery builds a SQL query for table query
func dbBuildSqlTableQuery(where map[string]string, resultColumns []string, schemaName string, tableName string, dbdialect sqldb.TDBDialect, permissionSqlStr string) (sqlStr string, sqlVals []interface{}, err error) {

	//check resultColumns
	if len(resultColumns) > 0 {
		err = checkSQLColumnsIsNoInjection(resultColumns)
		if err != nil {
			return "", nil, fmt.Errorf("check resultColumns err: %w", err)
		}
	}

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
