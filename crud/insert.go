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

// DbInsertReturn insert a message to db and return the inserted message
func DbInsertReturn(db *sql.DB, msg proto.Message, dbschema string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbInsertReturn(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsert insert a message to db
func DbInsert(db *sql.DB, msg proto.Message, dbschema string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbInsert(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsertWithTableNameReturn insert a message to db with table name and return the inserted message
func DbInsertWithTableNameReturn(db *sql.DB, msg proto.Message, dbschema string, tableName string) (returnMsg proto.Message, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	return dbInsertReturn(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)
}

// DbInsertWithTableName insert a message to db with table name
func DbInsertWithTableName(db *sql.DB, msg proto.Message, dbschema string, tableName string) (dmlResult *protodb.DMLResult, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	return dbInsert(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)
}

// dbInsertReturn insert a message to db and return the inserted message
func dbInsertReturn(db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (returnMsg proto.Message, err error) {

	returnInserted := true
	placeholder, _ := sqldb.GetDBPlaceholderCache(db)

	sqlStr, sqlVals, err := dbBuildSqlInsert(msg, dbschema, tableName, msgDesc, msgFieldDescs, placeholder, returnInserted)
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

	err = DbScan2ProtoMsg(rows, returnMsg, nil)

	return returnMsg, err
}

// dbInsert insert a message to db
func dbInsert(db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors) (dmlResult *protodb.DMLResult, err error) {

	returnAll := true
	placeholder, _ := sqldb.GetDBPlaceholderCache(db)

	sqlStr, sqlVals, err := dbBuildSqlInsert(msg, tableName, msgDesc, msgFieldDescs, placeholder, returnAll)
	if err != nil {
		return nil, err
	}

	if returnAll {

		rows, err := db.Query(sqlStr, sqlVals...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		if !rows.Next() {
			return nil, sql.ErrNoRows
		}

		columnNames, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		rowVals := make([]interface{}, len(columnNames))
		for i := range rowVals {
			rowVals[i] = new(interface{})
		}
		err = rows.Scan(rowVals...)
		if err != nil {
			fmt.Println("scan err:", err)
			return nil, err
		}

	} // else {
	//	sqlResult, err := db.Exec(sqlStr, sqlVals...)
	//	if err != nil {
	//		return nil, err
	//	}
	//	rowsAffected, err := sqlResult.RowsAffected()
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//}

}

func dbBuildSqlInsert(msgobj proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	placeholder protosql.SQLPlaceholder, returnInserted bool) (sqlStr string, vals []interface{}, err error) {
	sb := &strings.Builder{}
	sb.WriteString(protosql.SQL_INSERT_INTO)
	if len(dbschema) == 0 {
		sb.WriteString(tableName)
	} else {
		sb.WriteString(dbschema)
		sb.WriteString(protosql.SQL_DOT)
		sb.WriteString(tableName)

	}
	sb.WriteString(protosql.SQL_LEFT_PARENTHESES)
	firstCoumn := true
	columntCount := 0

	for fi := 0; fi < msgFieldDescs.Len(); fi++ {
		field := msgFieldDescs.Get(fi)

		fieldPdb := pdbutil.GetPDB(field)

		if !fieldPdb.NeedInInsert() {
			continue
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
		if !fieldPdb.IsNotNull() && (fieldPdb.IsReference() || fieldPdb.IsZeroAsNull()) {
			val = pdbutil.MaybeNull(val, field, fieldPdb)
		}
		vals = append(vals, val)
	}
	sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
	sb.WriteString(protosql.SQL_INSERT_VALUES)
	sb.WriteString(protosql.SQL_LEFT_PARENTHESES)

	firstPlaceholder := true

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
