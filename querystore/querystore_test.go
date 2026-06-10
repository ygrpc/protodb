package querystore

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/sqldb"
)

func testQueryFn(http.Header, sqldb.DB, *protodb.QueryReq) (string, []interface{}, msgstore.TFnGetMsg, error) {
	return "", nil, nil, nil
}

func TestQueryStoreConcurrentRegisterAndGet(t *testing.T) {
	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			RegisterQuery(fmt.Sprintf("query_%d", i), testQueryFn)
		}()
		go func() {
			defer wg.Done()
			GetQuery(fmt.Sprintf("query_%d", i))
		}()
	}

	wg.Wait()

	for i := 0; i < workers; i++ {
		if fn, ok := GetQuery(fmt.Sprintf("query_%d", i)); !ok || fn == nil {
			t.Fatalf("GetQuery query_%d = (%v, %v), want registered query", i, fn, ok)
		}
	}
}
