package querystore

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/sqldb"
)

type TfnQuerySqlGenerator func(meta http.Header, db sqldb.DB, req *protodb.QueryReq) (sqlStr string, sqlVals []interface{},
	fnGetResultMsg msgstore.TFnGetMsg, err error)

var (
	queryStoreMu sync.RWMutex
	queryStore   = make(map[string]TfnQuerySqlGenerator)
)

// RegisterQuery register a query to queryStore
// should call in init() function or at the beginning of the program before any query is used
// queryName is the name of the query
// queryFn is the function to generate sql

func RegisterQuery(queryName string, queryFn TfnQuerySqlGenerator) {
	queryStoreMu.RLock()
	oldQueryFn, ok := queryStore[queryName]
	queryStoreMu.RUnlock()
	if ok {
		fmt.Println("reregister query to queryStore:", queryName, "old:", oldQueryFn, "new:", queryFn)
	}
	queryStoreMu.Lock()
	queryStore[queryName] = queryFn
	queryStoreMu.Unlock()
}

// GetQuery get a query from queryStore
func GetQuery(queryName string) (TfnQuerySqlGenerator, bool) {
	queryStoreMu.RLock()
	queryFn, ok := queryStore[queryName]
	queryStoreMu.RUnlock()
	if !ok {
		return nil, false
	}
	return queryFn, true
}
