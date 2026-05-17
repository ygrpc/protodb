package msgstore

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
)

func testMsgFactory(new bool) proto.Message {
	return &protodb.QueryResp{}
}

func TestMsgStoreConcurrentRegisterAndGet(t *testing.T) {
	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			RegisterMsg(fmt.Sprintf("msg_%d", i), testMsgFactory)
		}()
		go func() {
			defer wg.Done()
			GetMsg(fmt.Sprintf("msg_%d", i), true)
		}()
	}

	wg.Wait()

	for i := 0; i < workers; i++ {
		if msg, ok := GetMsg(fmt.Sprintf("msg_%d", i), true); !ok || msg == nil {
			t.Fatalf("GetMsg msg_%d = (%v, %v), want registered message", i, msg, ok)
		}
	}
}

func TestFieldMsgStoreConcurrentRegisterAndGet(t *testing.T) {
	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			RegisterFieldMsg(fmt.Sprintf("field_msg_%d", i), testMsgFactory)
		}()
		go func() {
			defer wg.Done()
			GetFieldMsg(fmt.Sprintf("field_msg_%d", i), true)
		}()
	}

	wg.Wait()

	for i := 0; i < workers; i++ {
		if msg, ok := GetFieldMsg(fmt.Sprintf("field_msg_%d", i), true); !ok || msg == nil {
			t.Fatalf("GetFieldMsg field_msg_%d = (%v, %v), want registered message", i, msg, ok)
		}
	}
}
