package crud

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"

	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DbUpdatePartial update a message in db
func DbUpdatePartial(db *sql.DB, msg proto.Message, updateFields []string, dbschema string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdatePartial(db, msg, updateFields, dbschema, tableName, msgDesc, msgFieldDescs)

}

// dbUpdatePartial update a message in db
func dbUpdatePartial(db *sql.DB, msg proto.Message, updateFields []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (dmlResult *protodb.DMLResult, err error) {

	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdatePartial(msg, updateFields, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, false)
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

// DbUpdatePartialReturnNew update a message in db and return the updated message
func DbUpdatePartialReturnNew(db *sql.DB, msg proto.Message, updateFields []string, dbschema string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdatePartialReturnNew(db, msg, updateFields, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbUpdatePartialReturnNew update a message in db and return the updated message
func dbUpdatePartialReturnNew(db *sql.DB, msg proto.Message, updateFields []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (returnMsg proto.Message, err error) {

	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdatePartial(msg, updateFields, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, true)
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

	msgFieldsMap := pdbutil.BuildMsgFieldsMap(nil, msgDesc.Fields(), true)

	err = DbScan2ProtoMsg(rows, returnMsg, nil, msgFieldsMap)

	return returnMsg, err
}

// DbUpdatePartialReturnOldAndNew update a message in db and return both old and new messages
func DbUpdatePartialReturnOldAndNew(db *sql.DB, msg proto.Message, updateFields []string, dbschema string) (oldMsg proto.Message, newMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdatePartialReturnOldAndNew(db, msg, updateFields, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbUpdatePartialReturnOldAndNew updates a message in db and returns both old and new messages
func dbUpdatePartialReturnOldAndNew(db *sql.DB, msg proto.Message, updateFields []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (oldMsg proto.Message, newMsg proto.Message, err error) {

	dbdialect := sqldb.GetDBDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdatePartialOldAndNew(msg, updateFields, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Query(sqlStr, sqlVals...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil, sql.ErrNoRows
	}

	oldMsg = msg.ProtoReflect().New().Interface()
	newMsg = msg.ProtoReflect().New().Interface()

	msgFieldsMap := pdbutil.BuildMsgFieldsMap(nil, msgDesc.Fields(), true)

	err = DbScan2ProtoMsgx2(rows, oldMsg, newMsg, nil, msgFieldsMap)

	return oldMsg, newMsg, err
}

// dbBuildSqlUpdatePartial build sql update statement
func dbBuildSqlUpdatePartial(msgobj proto.Message, updateFields []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, returnUpdated bool) (sqlStr string, sqlVals []interface{}, err error) {

	sb := strings.Builder{}
	sb.WriteString(protosql.SQL_UPDATE)

	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)
	sb.WriteString(dbtableName)

	sb.WriteString(protosql.SQL_SET)

	valFieldNames := make([]string, 0)

	firstPlaceholder := true
	sqlParaNo := 1
	placeholder := dbdialect.Placeholder()
	primaryKeyFieldNames := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)

	if len(primaryKeyFieldNames) == 0 {
		return "", nil, fmt.Errorf("no primary key field")
	}

	updateFieldsMap := make(map[string]bool)
	for _, fieldName := range updateFields {
		updateFieldsMap[fieldName] = true
	}

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)

		fieldName := string(field.Name())

		if _, ok := updateFieldsMap[fieldName]; !ok {
			continue
		}

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

		isValZero := pdbutil.IsZeroValue(val)
		_, hasDefaultValue := fieldPdb.HasDefaultValue()
		if !fieldPdb.IsNotNull() && (fieldPdb.IsReference() || fieldPdb.IsZeroAsNull()) && isValZero {
			val = pdbutil.NullValue
		} else if isValZero && hasDefaultValue {
			val = fieldPdb.DefaultValue2SQLArgs()
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

	if returnUpdated {
		sb.WriteString(" RETURNING * ")
	}

	sb.WriteString(protosql.SQL_SEMICOLON)

	sqlStr = sb.String()

	return sqlStr, sqlVals, nil

}

// dbBuildSqlUpdatePartialOldAndNew build sql update statement and return old and new row
func dbBuildSqlUpdatePartialOldAndNew(msgobj proto.Message, updateFields []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect) (sqlStr string, sqlVals []interface{}, err error) {

	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)

	placeholder := dbdialect.Placeholder()
	primaryKeyFieldNames := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)

	if len(primaryKeyFieldNames) == 0 {
		return "", nil, fmt.Errorf("no primary key field")
	}

	sb := strings.Builder{}
	sb.WriteString("with old as (select * from ")
	sb.WriteString(dbtableName)
	sb.WriteString(protosql.SQL_WHERE)

	firstPlaceholder := true
	sqlParaNo := 1

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

	sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)

	sb.WriteString(protosql.SQL_UPDATE)
	sb.WriteString(tableName)
	sb.WriteString(" new ")
	sb.WriteString(protosql.SQL_SET)

	valFieldNames := make([]string, 0)
	updateFieldsMap := make(map[string]bool)
	for _, fieldName := range updateFields {
		updateFieldsMap[fieldName] = true
	}

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)
		fieldName := string(field.Name())

		if _, ok := updateFieldsMap[fieldName]; !ok {
			continue
		}

		if _, ok := primaryKeyFieldNames[fieldName]; ok {
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

		isValZero := pdbutil.IsZeroValue(val)
		_, hasDefaultValue := fieldPdb.HasDefaultValue()
		if !fieldPdb.IsNotNull() && (fieldPdb.IsReference() || fieldPdb.IsZeroAsNull()) && isValZero {
			val = pdbutil.NullValue
		} else if isValZero && hasDefaultValue {
			val = fieldPdb.DefaultValue2SQLArgs()
		}

		sqlVals = append(sqlVals, val)
	}

	if len(valFieldNames) == 0 {
		return "", nil, fmt.Errorf("no field need update")
	}

	firstPlaceholder = true
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

	sb.WriteString(protosql.SQL_FROM)
	sb.WriteString(" old ")
	sb.WriteString(protosql.SQL_WHERE)

	firstPlaceholder = true
	for fieldName := range primaryKeyFieldNames {
		if firstPlaceholder {
			firstPlaceholder = false
		} else {
			sb.WriteString(protosql.SQL_AND)
		}

		sb.WriteString(" new.")
		sb.WriteString(fieldName)
		sb.WriteString(protosql.SQL_EQUEAL)
		sb.WriteString("old.")
		sb.WriteString(fieldName)
	}

	sb.WriteString(" RETURNING old.*,new.* ;")

	return sb.String(), sqlVals, nil
}
