package crud

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strconv"
	"strings"
)

// DbDeleteRteurn delete a message from db and return the deleted message
func DbDeleteReturn(db *sql.DB, msg proto.Message, dbschema string) (returnMsg proto.Message, err error) {

	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbDeleteReturn(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)

}

func dbDeleteReturn(db *sql.DB, msg proto.Message, dbschema string, tableName string, msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors) (returnMsg proto.Message, err error) {
	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlDelete(msg, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, true)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(sqlStr, sqlVals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	returnMsg = msg.ProtoReflect().New().Interface()

	err = DbScan2ProtoMsg(rows, returnMsg, nil, nil)

	return returnMsg, err
}

// DbDelete delete a message from db
func DbDelete(db *sql.DB, msg proto.Message, dbschema string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlDelete(msg, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, false)
	if err != nil {
		return nil, err
	}

	sqlResult, err := db.Exec(sqlStr, sqlVals...)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return nil, err
	}

	dmlResult = &protodb.DMLResult{
		RowsAffected: rowsAffected,
	}

	return dmlResult, nil

}

// dbBuildSqlDelete build sql delete statement
func dbBuildSqlDelete(msgobj proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, returnDeleted bool) (sqlStr string, vals []interface{}, err error) {
	sb := &strings.Builder{}
	sb.WriteString(protosql.SQL_DELETE)
	sb.WriteString(protosql.SQL_FROM)

	if len(tableName) == 0 {
		tableName = string(msgDesc.Name())
	}

	if len(dbschema) == 0 {
		sb.WriteString(tableName)
	} else {
		switch dbdialect {
		case sqldb.Postgres, sqldb.Oracle:
			sb.WriteString(dbschema)
			sb.WriteString(protosql.SQL_DOT)
			sb.WriteString(tableName)

		default:
			sb.WriteString(dbschema + tableName)
		}

	}

	sb.WriteString(protosql.SQL_WHERE)

	firstPlaceholder := true
	sqlParaNo := 1
	placeholder := dbdialect.Placeholder()
	primaryKeyFieldNames := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)

	if len(primaryKeyFieldNames) == 0 {
		return "", nil, fmt.Errorf("no primary key field found for table %s", tableName)
	}

	for _, fieldDesc := range primaryKeyFieldNames {
		if firstPlaceholder {
			firstPlaceholder = false
		} else {
			sb.WriteString(protosql.SQL_AND)
		}

		sb.WriteString(fieldDesc.TextName())
		sb.WriteString(protosql.SQL_EQUEAL)
		if placeholder == protosql.SQL_QUESTION {
			sb.WriteString(string(protosql.SQL_QUESTION))
		} else {
			sb.WriteString(string(protosql.SQL_DOLLAR))
			sb.WriteString(strconv.Itoa(sqlParaNo))
			sqlParaNo++
		}

		val, err := pdbutil.GetField(msgobj, fieldDesc.TextName())
		if err != nil {
			return "", nil, err
		}
		vals = append(vals, val)

	}

	if returnDeleted {
		sb.WriteString(protosql.SQL_RETURNING)
		sb.WriteString(protosql.SQL_SPACE)
		sb.WriteString(protosql.SQL_ASTERISK)

	}
	sb.WriteString(protosql.SQL_SEMICOLON)

	return sb.String(), vals, nil
}
