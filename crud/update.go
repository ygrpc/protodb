package crud

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"strings"

	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DbUpdate update a message in db
func DbUpdate(db *sql.DB, msg proto.Message, dbschema string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdate(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)

}

// dbUpdate update a message in db
func dbUpdate(db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (dmlResult *protodb.DMLResult, err error) {

	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdate(msg, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, false)
	if err != nil {
		return nil, err

	}

	result, err := db.Exec(sqlStr, sqlVals...)
	if err != nil {
		return nil, err

	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err

	}

	dmlResult = &protodb.DMLResult{
		RowsAffected: rowsAffected,
	}

	return dmlResult, nil
}

// DbUpdateReturn update a message in db and return the updated message
func DbUpdateReturn(db *sql.DB, msg proto.Message, dbschema string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdateReturn(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbUpdateReturn update a message in db and return the updated message
func dbUpdateReturn(db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (returnMsg proto.Message, err error) {

	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdate(msg, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, true)
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

// dbBuildSqlUpdate build sql update statement
func dbBuildSqlUpdate(msgobj proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, returnUpdated bool) (sqlStr string, sqlVals []interface{}, err error) {

	sb := strings.Builder{}
	sb.WriteString(protosql.SQL_UPDATE)

	if len(dbschema) == 0 {
		sb.WriteString(tableName)
	} else {
		switch dbdialect {
		case sqldb.Postgres, sqldb.Oracle:
			sb.WriteString(dbschema)
			sb.WriteString(".")
			sb.WriteString(tableName)
		default:
			sb.WriteString(dbschema + tableName)
		}
	}

	sb.WriteString(protosql.SQL_SET)

	valFieldNames := make([]string, 0)

	firstPlaceholder := true
	sqlParaNo := 1
	placeholder := dbdialect.Placeholder()
	primaryKeyFieldNames := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)

	if len(primaryKeyFieldNames) == 0 {
		return "", nil, fmt.Errorf("no primary key field")
	}

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)

		fieldName := string(field.Name())

		if _, ok := primaryKeyFieldNames[fieldName]; ok {
			//primary key field, skip
			continue
		}

		fieldPdb, _ := pdbutil.GetPDB(field)

		if !fieldPdb.NeedInUpdate() {
			continue
		}

		valFieldNames = append(valFieldNames, fieldName)

		val, err := pdbutil.GetField(msgobj, fieldName)
		if err != nil {
			err = fmt.Errorf("get field err: %s.%s %w", msgDesc.Name(), fieldName, err)
			return "", nil, err
		}

		sqlVals = append(sqlVals, val)

	}

	if len(valFieldNames) == 0 {
		return "", nil, fmt.Errorf("no field need update")
	}

	for _, fieldName := range valFieldNames {
		if firstPlaceholder {
			firstPlaceholder = false
		} else {
			sb.WriteString(protosql.SQL_COMMA)
		}

		sb.WriteString(fieldName)
		sb.WriteString(protosql.SQL_EQUEAL)

		if placeholder == protosql.SQL_QUESTION {
			sb.WriteString(string(protosql.SQL_QUESTION))
		} else {
			sb.WriteString(string(protosql.SQL_DOLLAR))
			sb.WriteString(fmt.Sprint(sqlParaNo))
			sqlParaNo++
		}
	}

	sb.WriteString(protosql.SQL_WHERE)

	firstPlaceholder = true

	for fieldName := range primaryKeyFieldNames {
		if firstPlaceholder {
			firstPlaceholder = false
		} else {
			sb.WriteString(protosql.SQL_AND)
		}

		sb.WriteString(fieldName)
		sb.WriteString(protosql.SQL_EQUEAL)

		if placeholder == protosql.SQL_QUESTION {
			sb.WriteString(string(protosql.SQL_QUESTION))
		} else {
			sb.WriteString(string(protosql.SQL_DOLLAR))
			sb.WriteString(fmt.Sprint(sqlParaNo))
			sqlParaNo++
		}

		val, err := pdbutil.GetField(msgobj, fieldName)
		if err != nil {
			return "", nil, err
		}
		sqlVals = append(sqlVals, val)
	}

	sqlStr = sb.String()

	if returnUpdated {
		sb.WriteString(protosql.SQL_RETURNING)
		sb.WriteString(protosql.SQL_SPACE)
		sb.WriteString(protosql.SQL_ASTERISK)
	}

	sqlStr += protosql.SQL_SEMICOLON

	return sqlStr, sqlVals, nil

}
