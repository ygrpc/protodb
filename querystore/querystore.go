package querystore

import (
	"database/sql"
	"fmt"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/msgstore"
	"net/http"
)

type TfnQuerySqlGenerator func(meta http.Header, db *sql.DB, req *protodb.QueryReq) (sqlStr string, sqlVals []interface{},
	fnGetResultMsg msgstore.TFnGetMsg, err error)

var queryStore = make(map[string]TfnQuerySqlGenerator)

// RegisterQuery register a query to queryStore
// should call in init() function or at the beginning of the program before any query is used
// queryName is the name of the query
// queryFn is the function to generate sql

func RegisterQuery(queryName string, queryFn TfnQuerySqlGenerator) {
	if oldQueryFn, ok := queryStore[queryName]; ok {
		fmt.Println("reregister query to queryStore:", queryName, "old:", oldQueryFn, "new:", queryFn)
	}
	queryStore[queryName] = queryFn
}

// GetQuery get a query from queryStore
func GetQuery(queryName string) (TfnQuerySqlGenerator, bool) {
	queryFn, ok := queryStore[queryName]
	if !ok {
		return nil, false
	}
	return queryFn, true
}
