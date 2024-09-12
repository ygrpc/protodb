package crud

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DbInsertReturn insert a message to db and return the inserted message
func DbInsertReturn(db *sql.DB, msg proto.Message, msgLastFieldNo int32, dbschema string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbInsertReturn(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsert insert a message to db
func DbInsert(db *sql.DB, msg proto.Message, msgLastFieldNo int32, dbschema string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbInsert(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsertWithTableNameReturn insert a message to db with table name and return the inserted message
func DbInsertWithTableNameReturn(db *sql.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	return dbInsertReturn(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsertWithTableName insert a message to db with table name
func DbInsertWithTableName(db *sql.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	return dbInsert(db, msg, msgLastFieldNo, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbInsertReturn insert a message to db and return the inserted message
func dbInsertReturn(db *sql.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (returnMsg proto.Message, err error) {

	dbdialect := sqldb.GetDBDialect(db)

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

	err = DbScan2ProtoMsg(rows, returnMsg, nil, nil)

	return returnMsg, err
}

// dbInsert insert a message to db
func dbInsert(db *sql.DB, msg proto.Message, msgLastFieldNo int32, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (dmlResult *protodb.DMLResult, err error) {

	dbdialect := sqldb.GetDBDialect(db)

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

	dmlResult = &protodb.DMLResult{
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
		_, hasDefaultValue := fieldPdb.HasDefaultValue()
		if !fieldPdb.IsNotNull() && (fieldPdb.IsReference() || fieldPdb.IsZeroAsNull()) && isValZero {
			val = pdbutil.NullValue
		} else if isValZero && hasDefaultValue {
			val = fieldPdb.DefaultValue2SQLArgs()
		}
		vals = append(vals, val)
	}
	sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
	sb.WriteString(protosql.SQL_INSERT_VALUES)
	sb.WriteString(protosql.SQL_LEFT_PARENTHESES)

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
		sb.WriteString(protosql.SQL_RETURNING)
		sb.WriteString(protosql.SQL_SPACE)
		sb.WriteString(protosql.SQL_ASTERISK)

	}
	sb.WriteString(protosql.SQL_SEMICOLON)

	return sb.String(), vals, nil
}
