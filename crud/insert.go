package crud

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DbInsertReturn insert a message to db and return the inserted message
// db can be *sql.DB, *sql.Tx or sqldb.DBExecutor for transaction support
func DbInsertReturn(db sqldb.DBExecutor, msg proto.Message, msgLastFieldNo int32, dbschema string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbInsertReturn(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsert insert a message to db
// db can be *sql.DB, *sql.Tx or sqldb.DBExecutor for transaction support
func DbInsert(db sqldb.DBExecutor, msg proto.Message, msgLastFieldNo int32, dbschema string) (dmlResult *protodb.CrudResp, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbInsert(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsertWithTableNameReturn insert a message to db with table name and return the inserted message
// db can be *sql.DB, *sql.Tx or sqldb.DBExecutor for transaction support
func DbInsertWithTableNameReturn(db sqldb.DBExecutor, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	return dbInsertReturn(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsertWithTableName insert a message to db with table name
// db can be *sql.DB, *sql.Tx or sqldb.DBExecutor for transaction support
func DbInsertWithTableName(db sqldb.DBExecutor, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string) (dmlResult *protodb.CrudResp, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	return dbInsert(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbInsertReturn insert a message to db and return the inserted message
func dbInsertReturn(db sqldb.DBExecutor, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (returnMsg proto.Message, err error) {

	dbdialect := sqldb.GetExecutorDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlInsert(msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, true)
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

// dbInsert insert a message to db
func dbInsert(db sqldb.DBExecutor, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (dmlResult *protodb.CrudResp, err error) {

	dbdialect := sqldb.GetExecutorDialect(db)

	sqlStr, sqlVals, err := dbBuildSqlInsert(msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, false)
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

	dmlResult = &protodb.CrudResp{
		RowsAffected: rowsAffected,
	}

	return dmlResult, nil

}

func dbBuildSqlInsert(msgobj proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, returnInserted bool) (sqlStr string, vals []interface{}, err error) {
	sb := &strings.Builder{}
	sb.WriteString(protosql.SQL_INSERT_INTO)

	if len(tableName) == 0 {
		tableName = string(msgDesc.Name())
	}

	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)
	sb.WriteString(dbtableName)
	sb.WriteString(protosql.SQL_LEFT_PARENTHESES)
	firstCoumn := true
	columntCount := 0

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)

		fieldPdb, _ := pdbutil.GetPDB(field)

		if !fieldPdb.NeedInInsert() {
			continue
		}

		if msgLastFieldNo > 0 {
			if int32(field.Number()) > msgLastFieldNo {
				if fieldPdb.DefaultValue != "" {
					continue
				} else {
					err = fmt.Errorf("field %s.%s is not set and has no default value", msgDesc.Name(), string(field.Name()))
					return "", nil, err
				}
			}
		}

		fieldName := string(field.Name())

		val, err := pdbutil.GetField(msgobj, fieldName)
		if err != nil {
			err = fmt.Errorf("get field err: %s.%s %w", msgDesc.Name(), fieldName, err)
			return "", nil, err
		}

		if firstCoumn {
			firstCoumn = false
		} else {
			sb.WriteString(protosql.SQL_COMMA)
		}
		columntCount++
		sb.WriteString(fieldName)
		isValZero := pdbutil.IsZeroValue(val)
		hasSetDefaultValue := false
		_, hasDefaultValue := fieldPdb.HasDefaultValue()
		if !fieldPdb.IsNotNull() && (fieldPdb.IsReference() || fieldPdb.IsZeroAsNull()) && isValZero {
			val = pdbutil.NullValue
			hasSetDefaultValue = true
		} else if isValZero && hasDefaultValue {
			val = fieldPdb.DefaultValue2SQLArgs()
			hasSetDefaultValue = true
		}
		if field.Kind() == protoreflect.MessageKind && !hasSetDefaultValue {
			b, err := protojson.Marshal(val.(proto.Message))
			if err != nil {
				return "", nil, fmt.Errorf("marshal msg:%s field:%s msg to json err: %s", msgDesc.Name(), fieldName, err.Error())
			}
			val = string(b)
		}
		vals = append(vals, val)
	}
	//sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
	//sb.WriteString(protosql.SQL_INSERT_VALUES)
	//sb.WriteString(protosql.SQL_LEFT_PARENTHESES)
	sb.WriteString(" ) VALUES ( ")

	firstPlaceholder := true

	placeholder := dbdialect.Placeholder()

	for i := 0; i < columntCount; i++ {
		if firstPlaceholder {
			firstPlaceholder = false
		} else {
			sb.WriteString(protosql.SQL_COMMA)
		}
		if placeholder == protosql.SQL_QUESTION {
			sb.WriteString(string(protosql.SQL_QUESTION))
		} else {
			sb.WriteString(string(protosql.SQL_DOLLAR))
			sb.WriteString(strconv.Itoa(i + 1))
		}
	}

	sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
	if returnInserted {
		sb.WriteString(" RETURNING * ")

	}
	sb.WriteString(protosql.SQL_SEMICOLON)

	return sb.String(), vals, nil
}
