package service

import (
	"net/http"
	"testing"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

func newTestBroadcaster() *TcrudBroadcaster {
	return &TcrudBroadcaster{
		fnCrudBroadcastMap:       xsync.NewMapOf[string, []TfnCrudBroadcastHandler](),
		fnCrudBroadcastByCodeMap: xsync.NewMapOf[protodb.CrudReqCode, *xsync.MapOf[string, []TfnCrudBroadcastHandler]](),
	}
}

func testBroadcastReq(table string, code protodb.CrudReqCode) *protodb.CrudReq {
	return &protodb.CrudReq{
		TableName: table,
		Code:      code,
	}
}

func TestUnregisterBroadcastRemovesHandler(t *testing.T) {
	broadcaster := newTestBroadcaster()
	calls := 0
	handler := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		calls++
	}

	broadcaster.RegisterBroadcast("User", handler)
	broadcaster.Broadcast(nil, nil, testBroadcastReq("User", protodb.CrudReqCode_INSERT), nil, nil)
	if calls != 1 {
		t.Fatalf("calls after register = %d, want 1", calls)
	}

	broadcaster.UnregisterBroadcast("User", handler)
	broadcaster.Broadcast(nil, nil, testBroadcastReq("User", protodb.CrudReqCode_INSERT), nil, nil)
	if calls != 1 {
		t.Fatalf("calls after unregister = %d, want 1", calls)
	}
}

func TestUnregisterBroadcastByCodeRemovesHandler(t *testing.T) {
	broadcaster := newTestBroadcaster()
	calls := 0
	handler := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		calls++
	}

	broadcaster.RegisterBroadcastByCode("User", protodb.CrudReqCode_UPDATE, handler)
	broadcaster.Broadcast(nil, nil, testBroadcastReq("User", protodb.CrudReqCode_UPDATE), nil, nil)
	if calls != 1 {
		t.Fatalf("calls after register = %d, want 1", calls)
	}

	broadcaster.UnregisterBroadcastByCode("User", protodb.CrudReqCode_UPDATE, handler)
	broadcaster.Broadcast(nil, nil, testBroadcastReq("User", protodb.CrudReqCode_UPDATE), nil, nil)
	if calls != 1 {
		t.Fatalf("calls after unregister = %d, want 1", calls)
	}
}

func TestUnregisterBroadcastLeavesOtherHandlers(t *testing.T) {
	broadcaster := newTestBroadcaster()
	callsA := 0
	callsB := 0
	handlerA := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		callsA++
	}
	handlerB := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		callsB++
	}

	broadcaster.RegisterBroadcast("User", handlerA)
	broadcaster.RegisterBroadcast("User", handlerB)
	broadcaster.UnregisterBroadcast("User", handlerA)
	broadcaster.Broadcast(nil, nil, testBroadcastReq("User", protodb.CrudReqCode_INSERT), nil, nil)

	if callsA != 0 {
		t.Fatalf("callsA = %d, want 0", callsA)
	}
	if callsB != 1 {
		t.Fatalf("callsB = %d, want 1", callsB)
	}
}
