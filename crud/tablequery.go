package crud

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

type TqueryItem struct {
	Err   *string
	IsEnd bool
	Msg   proto.Message
}

func TableQueryBuildSql(db *sql.DB, tableQueryReq *protodb.TableQueryReq, permissionSqlStr string) (sqlStr string, sqlVals []interface{}, err error) {
	// Check result columns
	if len(tableQueryReq.ResultColumnNames) > 0 {
		err = checkSQLColumnsIsNoInjection(tableQueryReq.ResultColumnNames)
		if err != nil {
			return "", nil, fmt.Errorf("check resultColumns err: %w", err)
		}
	}

	sb := strings.Builder{}
	sb.WriteString(protosql.SQL_SELECT)

	// Handle result columns
	if len(tableQueryReq.ResultColumnNames) == 0 {
		sb.WriteString(protosql.SQL_ASTERISK)
	} else {
		sb.WriteString(strings.Join(tableQueryReq.ResultColumnNames, protosql.SQL_COMMA))
	}

	sb.WriteString(protosql.SQL_FROM)

	// Build table name
	dbdialect := sqldb.GetDBDialect(db)
	dbtableName := sqldb.BuildDbTableName(tableQueryReq.TableName, tableQueryReq.SchemeName, dbdialect)
	sb.WriteString(dbtableName)

	// Handle WHERE clauses
	placeholder := dbdialect.Placeholder()
	sqlParaNo := 1

	if len(tableQueryReq.Where) > 0 || len(permissionSqlStr) > 0 {

		sb.WriteString(protosql.SQL_WHERE)

		firstPlaceholder := true

		// Add permission SQL if provided
		if len(permissionSqlStr) > 0 {
			sb.WriteString(protosql.SQL_LEFT_PARENTHESES)
			sb.WriteString(permissionSqlStr)
			sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
			firstPlaceholder = false
		}

		// Add conditions from where map
		for fieldName, fieldValue := range tableQueryReq.Where {
			if !firstPlaceholder {
				sb.WriteString(protosql.SQL_AND)
			}
			firstPlaceholder = false

			//check fieldname security
			err = checkSQLColumnsIsNoInjectionStr(fieldName)
			if err != nil {
				return "", nil, fmt.Errorf("check fieldname %s err: %w", fieldName, err)
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

			sqlVals = append(sqlVals, fieldValue)
		}
	}

	// Add LIMIT and OFFSET if specified
	if tableQueryReq.Limit > 0 {
		sb.WriteString(protosql.SQL_LIMIT)
		sb.WriteString(fmt.Sprint(tableQueryReq.Limit))
	}

	if tableQueryReq.Offset > 0 {
		sb.WriteString(protosql.SQL_OFFSET)
		sb.WriteString(fmt.Sprint(tableQueryReq.Offset))
	}

	sqlStr = sb.String()
	return sqlStr, sqlVals, nil
}
