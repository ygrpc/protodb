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

	firstPlaceholder := true

	if len(tableQueryReq.Where) > 0 || len(permissionSqlStr) > 0 {

		sb.WriteString(protosql.SQL_WHERE)

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

	//handle where2
	if len(tableQueryReq.Where2) > 0 {
		if len(tableQueryReq.Where2) != len(tableQueryReq.Where2Operator) {
			return "", nil, fmt.Errorf("where2 and where2Operator must have same length")
		}

		if firstPlaceholder {
			sb.WriteString(protosql.SQL_WHERE)
		}

		for fieldname, fieldValue := range tableQueryReq.Where2 {
			fieldWhereOperator, ok := tableQueryReq.Where2Operator[fieldname]
			if !ok {
				return "", nil, fmt.Errorf("where2 field %s has no operator provided", fieldname)
			}

			//check fieldname security
			err = checkSQLColumnsIsNoInjectionStr(fieldname)
			if err != nil {
				return "", nil, fmt.Errorf("check fieldname %s err: %w", fieldname, err)
			}

			if firstPlaceholder {
				//has written where before
				firstPlaceholder = false
			} else {
				sb.WriteString(protosql.SQL_AND)
			}

			sb.WriteString(fieldname)
			sb.WriteString(WhereOperator2Str(fieldWhereOperator))

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

func WhereOperator2Str(fieldop protodb.WhereOperator) string {
	switch fieldop {
	case protodb.WhereOperator_WOP_GT:
		return protosql.SQL_GT
	case protodb.WhereOperator_WOP_LT:
		return protosql.SQL_LT
	case protodb.WhereOperator_WOP_GTE:
		return protosql.SQL_GTE
	case protodb.WhereOperator_WOP_LTE:
		return protosql.SQL_LTE
	case protodb.WhereOperator_WOP_LIKE:
		return protosql.SQL_LIKE
	case protodb.WhereOperator_WOP_EQ:
		return protosql.SQL_EQUEAL
	default:
		return " unsupported operator: " + fmt.Sprint(fieldop)
	}
}
