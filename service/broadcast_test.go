package service

import (
	"fmt"
	"net/http"
	"testing"
	"time"

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

func TestBroadcastRecoversHandlerPanic(t *testing.T) {
	broadcaster := newTestBroadcaster()
	calls := 0

	broadcaster.RegisterBroadcast("User", func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		panic("boom")
	})
	broadcaster.RegisterBroadcast("User", func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		calls++
	})

	broadcaster.Broadcast(nil, nil, testBroadcastReq("User", protodb.CrudReqCode_INSERT), nil, nil)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestBroadcastAsyncSnapshotsMutableInputs(t *testing.T) {
	broadcaster := newTestBroadcaster()
	errCh := make(chan error, 1)
	meta := http.Header{"X-Test": []string{"old"}}
	req := &protodb.CrudReq{
		TableName: "User",
		Code:      protodb.CrudReqCode_INSERT,
		MsgBytes:  []byte("old"),
	}
	reqMsg := &protodb.PDBField{Comment: []string{"old"}}
	respMsg := &protodb.CrudResp{
		RowsAffected: 1,
		NewMsgBytes:  []byte("old"),
	}

	broadcaster.RegisterBroadcast("User", func(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
		if got := meta.Get("X-Test"); got != "old" {
			errCh <- fmt.Errorf("meta X-Test = %q, want old", got)
			return
		}
		if req.TableName != "User" {
			errCh <- fmt.Errorf("req.TableName = %q, want User", req.TableName)
			return
		}
		if got := string(req.MsgBytes); got != "old" {
			errCh <- fmt.Errorf("req.MsgBytes = %q, want old", got)
			return
		}

		gotReqMsg, ok := reqMsg.(*protodb.PDBField)
		if !ok {
			errCh <- fmt.Errorf("reqMsg type = %T, want *protodb.PDBField", reqMsg)
			return
		}
		if gotReqMsg.Comment[0] != "old" {
			errCh <- fmt.Errorf("reqMsg.Comment[0] = %q, want old", gotReqMsg.Comment[0])
			return
		}

		gotRespMsg, ok := respMsg.(*protodb.CrudResp)
		if !ok {
			errCh <- fmt.Errorf("respMsg type = %T, want *protodb.CrudResp", respMsg)
			return
		}
		if gotRespMsg.RowsAffected != 1 {
			errCh <- fmt.Errorf("respMsg.RowsAffected = %d, want 1", gotRespMsg.RowsAffected)
			return
		}
		if got := string(gotRespMsg.NewMsgBytes); got != "old" {
			errCh <- fmt.Errorf("respMsg.NewMsgBytes = %q, want old", got)
			return
		}
		errCh <- nil
	})

	broadcaster.BroadcastAsync(meta, nil, req, reqMsg, respMsg)
	meta.Set("X-Test", "new")
	req.TableName = "Other"
	req.MsgBytes[0] = 'n'
	reqMsg.Comment[0] = "new"
	respMsg.RowsAffected = 2
	respMsg.NewMsgBytes[0] = 'n'

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("broadcast handler was not called")
	}
}
