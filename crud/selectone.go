package crud

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DbSelectOne select one message from db, if keyColumns is empty, use primary key fields as key columns, keyColumns need is unique
// if resultColumns is empty, use all fields as result columns
// limitPkUk: if true, limit primary key or unique key columns for select one, keyColumns should be unique
func DbSelectOne(db *sql.DB, msg proto.Message, keyColumns []string, resultColumns []string, dbschema string, limitPkUk bool) (returnMsg proto.Message, err error) {
	if len(keyColumns) > 0 {
		err = checkSQLColumnsIsNoInjection(keyColumns, ColumnNameCheckMethodStrict)
		if err != nil {
			return nil, err
		}
	}

	if len(resultColumns) > 0 {
		err = checkSQLColumnsIsNoInjection(resultColumns, ColumnNameCheckMethodInWhereOrResult)
		if err != nil {
			return nil, err
		}
	}

	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())
	dbdialect := sqldb.GetDBDialect(db)

	return dbSelectOne(db, msg, keyColumns, resultColumns, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, limitPkUk)
}

func dbSelectOne(db *sql.DB, msg proto.Message, keyColumns []string, resultColumns []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, limitPkUk bool) (returnMsg proto.Message, err error) {

	sqlStr, sqlVals, err := dbBuildSqlSelectOne(msg, keyColumns, resultColumns, dbschema, tableName, msgDesc, msgFieldDescs, dbdialect, limitPkUk)
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

	if len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*") {
		msgFieldsMap := pdbutil.BuildMsgFieldsMap(nil, msgDesc.Fields(), true)
		err = DbScan2ProtoMsg(rows, returnMsg, nil, msgFieldsMap)
	} else {
		msgFieldsMap := pdbutil.BuildMsgFieldsMap(resultColumns, msgDesc.Fields(), true)
		err = DbScan2ProtoMsg(rows, returnMsg, resultColumns, msgFieldsMap)
	}

	return returnMsg, err
}

func dbBuildSqlSelectOne(msg proto.Message, keyColumns []string, resultColumns []string, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors,
	dbdialect sqldb.TDBDialect, limitPkUk bool) (sqlStr string, sqlVals []interface{}, err error) {

	sb := strings.Builder{}
	sb.WriteString(protosql.SQL_SELECT)

	if len(resultColumns) == 0 {
		sb.WriteString(protosql.SQL_ASTERISK)
	} else {
		sb.WriteString(strings.Join(resultColumns, protosql.SQL_COMMA))
	}

	sb.WriteString(protosql.SQL_FROM)

	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)
	sb.WriteString(dbtableName)

	sb.WriteString(protosql.SQL_WHERE)

	if len(keyColumns) == 0 {
		primaryKeyFieldNames := pdbutil.GetPrimaryKeyFieldDescs(msgDesc, msgFieldDescs, false)
		keyColumns = make([]string, len(primaryKeyFieldNames))
		for fieldName := range primaryKeyFieldNames {
			keyColumns = append(keyColumns, fieldName)
		}

		if len(keyColumns) == 0 {
			return "", nil, fmt.Errorf("no primary key field for table %s", tableName)
		}
	} else {
		if limitPkUk {
			primaryOrUniqueKeyFieldNames := pdbutil.GetPrimaryKeyOrUniqueFieldDescs(msgDesc, msgFieldDescs, false)
			for _, fieldName := range keyColumns {
				constraint, ok := primaryOrUniqueKeyFieldNames[strings.ToLower(fieldName)]
				if !ok {
					return "", nil, fmt.Errorf("column %s:%s is not primary or unique key", tableName, fieldName)
				}
				if len(keyColumns) != len(constraint.Fields) {
					gotColumns := strings.Join(keyColumns, ",")
					needCoumns := ""
					for fieldName := range constraint.Fields {
						needCoumns += fieldName + ","
					}
					return "", nil, fmt.Errorf("not equal columns for %s, need [%s], got [%s]", constraint.PrimaryOrUniqueName, needCoumns, gotColumns)
				}

			}
		}
	}

	firstPlaceholder := true
	placeholder := dbdialect.Placeholder()
	sqlParaNo := 1

	for _, fieldName := range keyColumns {
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

		val, err := pdbutil.GetField(msg, fieldName)
		if err != nil {
			return "", nil, err
		}
		sqlVals = append(sqlVals, val)
	}

	sb.WriteString(protosql.SQL_LIMIT_1)

	sqlStr = sb.String()

	return sqlStr, sqlVals, nil
}
