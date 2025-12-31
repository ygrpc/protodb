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

// DbUpdate update a message in db
// db can be *sql.DB, *sql.Tx or sqldb.DB for transaction support
func DbUpdate(db sqldb.DB, msg proto.Message, msgLastFieldNo int32, dbschema string) (dmlResult *protodb.CrudResp, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdate(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbUpdate update a message in db
func dbUpdate(db sqldb.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
) (dmlResult *protodb.CrudResp, err error) {
	dbdialect := sqldb.GetExecutorDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdate(msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, false)
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

	dmlResult = &protodb.CrudResp{
		RowsAffected: rowsAffected,
	}

	return dmlResult, nil
}

// DbUpdateReturnNew update a message in db and return the updated message
// db can be *sql.DB, *sql.Tx or sqldb.DB for transaction support
func DbUpdateReturnNew(db sqldb.DB, msg proto.Message, msgLastFieldNo int32, dbschema string) (newMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdateReturnNew(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbUpdateReturnOldAndNew update a message in db and return both old and new messages
// db can be *sql.DB, *sql.Tx or sqldb.DB for transaction support
func DbUpdateReturnOldAndNew(db sqldb.DB, msg proto.Message, msgLastFieldNo int32, dbschema string) (oldMsg proto.Message, newMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbUpdateReturnOldAndNew(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbUpdateReturnNew update a message in db and return the updated message
func dbUpdateReturnNew(db sqldb.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
) (newMsg proto.Message, err error) {
	dbdialect := sqldb.GetExecutorDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlUpdate(msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, true)
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

	newMsg = msg.ProtoReflect().New().Interface()

	msgFieldsMap := pdbutil.BuildMsgFieldsMap(nil, msgDesc.Fields(), true)

	err = DbScan2ProtoMsg(rows, newMsg, nil, msgFieldsMap)

	return newMsg, err
}

// dbBuildSqlUpdate build sql update statement
func dbBuildSqlUpdate(msgobj proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, returnUpdated bool,
) (sqlStr string, sqlVals []interface{}, err error) {
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

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)

		fieldName := string(field.Name())

		if _, ok := primaryKeyFieldNames[fieldName]; ok {
			// primary key field, skip
			continue
		}

		fieldPdb, _ := pdbutil.GetPDB(field)

		if !fieldPdb.NeedInUpdate() {
			continue
		}

		if msgLastFieldNo > 0 {
			if int32(field.Number()) > msgLastFieldNo {
				continue
			}
		}

		valFieldNames = append(valFieldNames, fieldName)

		val, err := pdbutil.GetField(msgobj, fieldName)
		if err != nil {
			err = fmt.Errorf("get field err: %s.%s %w", msgDesc.Name(), fieldName, err)
			return "", nil, err
		}

		isValZero := pdbutil.IsZeroValue(val)
		_, hasDefaultValue := fieldPdb.HasDefaultValue()
		hasSetDefaultValue := false
		if !fieldPdb.IsNotNull() && (fieldPdb.IsReference() || fieldPdb.IsZeroAsNull()) && isValZero {
			val = pdbutil.NullValue
			hasSetDefaultValue = true
		} else if isValZero && hasDefaultValue {
			val = fieldPdb.DefaultValue2SQLArgs()
			hasSetDefaultValue = true
		}
		if !hasSetDefaultValue {
			val, err = EncodeSQLArg(field, dbdialect, val)
			if err != nil {
				return "", nil, fmt.Errorf("encode sql arg msg:%s field:%s err: %w", msgDesc.Name(), fieldName, err)
			}
		}

		sqlVals = append(sqlVals, val)
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

// dbUpdateReturnOldAndNew updates a message in db and returns both old and new messages
func dbUpdateReturnOldAndNew(db sqldb.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
) (oldMsg proto.Message, newMsg proto.Message, err error) {
	dbdialect := sqldb.GetExecutorDialect(db)

	//if db is sqlite, sqlite is not support return old and new,use selectone + returnnew
	if dbdialect == sqldb.SQLite {
		oldMsg, err = dbSelectOne(db, msg, nil, nil, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, true)
		if err != nil {
			return nil, nil, err
		}
		newMsg, err = dbUpdateReturnNew(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
		return oldMsg, newMsg, err
	}

	fnbuildsql := dbBuildSqlUpdateOldAndNew
	if dbdialect == sqldb.Postgres {
		pgversion, _ := GetPgVersion(db)
		if pgversion.Major >= 18 {

			fnbuildsql = dbBuildSqlUpdateOldAndNewNative
		}
	}
	if dbdialect == sqldb.Oracle {
		fnbuildsql = dbBuildSqlUpdateOldAndNewNative
	}

	sqlStr, sqlVals, err := fnbuildsql(msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect)
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

	return oldMsg, newMsg, nil
}

// dbBuildSqlUpdateOldAndNew build sql update statement and return old and new row
func dbBuildSqlUpdateOldAndNew(msgobj proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect,
) (sqlStr string, sqlVals []interface{}, err error) {
	// build the sql like below
	// with old as (select * from ttt where id=1)
	// update ttt new set username='1234567' from old where new.id=old.id RETURNING old.*,new.*;

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

	// sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
	// sb.WriteString(protosql.SQL_UPDATE)

	sb.WriteString(" ) UPDATE ")
	sb.WriteString(tableName)
	sb.WriteString(" new set ")
	// sb.WriteString(protosql.SQL_SET)

	valFieldNames := make([]string, 0)

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)

		fieldName := string(field.Name())

		if _, ok := primaryKeyFieldNames[fieldName]; ok {
			// primary key field, skip
			continue
		}

		fieldPdb, _ := pdbutil.GetPDB(field)

		if !fieldPdb.NeedInUpdate() {
			continue
		}

		if msgLastFieldNo > 0 {
			if int32(field.Number()) > msgLastFieldNo {
				continue
			}
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
		val, err = EncodeSQLArg(field, dbdialect, val)
		if err != nil {
			return "", nil, fmt.Errorf("encode sql arg msg:%s field:%s err: %w", msgDesc.Name(), fieldName, err)
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

	sb.WriteString(" from old where ")

	firstPlaceholder = true

	for fieldName := range primaryKeyFieldNames {
		if firstPlaceholder {
			firstPlaceholder = false
		} else {
			sb.WriteString(protosql.SQL_AND)
		}

		sb.WriteString(" new.")
		sb.WriteString(fieldName)
		// sb.WriteString(protosql.SQL_EQUEAL)
		sb.WriteString("=old.")
		sb.WriteString(fieldName)

	}

	sb.WriteString(" RETURNING old.*,new.* ;")

	sqlStr = sb.String()

	return sqlStr, sqlVals, nil
}

// dbBuildSqlUpdateOldAndNewNative build sql update statement and return old and new row
func dbBuildSqlUpdateOldAndNewNative(msgobj proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect,
) (sqlStr string, sqlVals []interface{}, err error) {
	// Build SQL like: UPDATE <table> SET ... WHERE pk=? RETURNING OLD.*,NEW.*;
	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)

	placeholder := dbdialect.Placeholder()
	primaryKeyFieldNames := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)

	if len(primaryKeyFieldNames) == 0 {
		return "", nil, fmt.Errorf("no primary key field")
	}

	sb := strings.Builder{}
	sb.WriteString(protosql.SQL_UPDATE)
	sb.WriteString(dbtableName)
	sb.WriteString(protosql.SQL_SET)

	valFieldNames := make([]string, 0)

	// Collect updatable fields and values
	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)
		fieldName := string(field.Name())

		if _, ok := primaryKeyFieldNames[fieldName]; ok {
			continue
		}

		fieldPdb, _ := pdbutil.GetPDB(field)
		if !fieldPdb.NeedInUpdate() {
			continue
		}

		if msgLastFieldNo > 0 && int32(field.Number()) > msgLastFieldNo {
			continue
		}

		valFieldNames = append(valFieldNames, fieldName)

		val, err := pdbutil.GetField(msgobj, fieldName)
		if err != nil {
			return "", nil, fmt.Errorf("get field err: %s.%s %w", msgDesc.Name(), fieldName, err)
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

	// Write SET clause placeholders
	firstPlaceholder := true
	sqlParaNo := 1
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

	// WHERE by primary keys
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

		// Append PK values after SET values
		val, err := pdbutil.GetField(msgobj, fieldName)
		if err != nil {
			return "", nil, err
		}
		sqlVals = append(sqlVals, val)
	}

	// Native returning old and new
	sb.WriteString(" RETURNING OLD.*,NEW.* ;")

	sqlStr = sb.String()
	return sqlStr, sqlVals, nil
}
