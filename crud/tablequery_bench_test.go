package crud

import (
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
)

var tableQuerySQLSink string
var tableQueryValsSink []interface{}

func BenchmarkTableQueryBuildSql_PostgresMixedWhere(b *testing.B) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()
	req := &protodb.TableQueryReq{
		TableName:         "tablequeryreq",
		ResultColumnNames: []string{"TableName", "Limit", "Offset"},
		Where: map[string]string{
			"tablename": "User",
			"limit":     "10",
		},
		Where2: map[string]string{
			"offset": "20",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"offset": protodb.WhereOperator_WOP_GT,
		},
		Limit:  50,
		Offset: 100,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "tenant_id=$1", []any{int64(7)})
		if err != nil {
			b.Fatal(err)
		}
		tableQuerySQLSink = sqlStr
		tableQueryValsSink = vals
	}
}
