package service

import (
	"net/http"
	"reflect"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

// TfnCrudBroadcastHandler broadcast handler
// meta http header
// db database connection
// req request message
// reqMsg request message
// respMsg response message, may be nil
type TfnCrudBroadcastHandler func(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message)

type TcrudBroadcaster struct {
	// msgName => fns
	fnCrudBroadcastMap *xsync.MapOf[string, []TfnCrudBroadcastHandler]
	// crudCode => {msgName => fns}
	fnCrudBroadcastByCodeMap *xsync.MapOf[protodb.CrudReqCode, *xsync.MapOf[string, []TfnCrudBroadcastHandler]]
}

var GlobalCrudBroadcaster *TcrudBroadcaster = &TcrudBroadcaster{
	fnCrudBroadcastMap:       xsync.NewMapOf[string, []TfnCrudBroadcastHandler](),
	fnCrudBroadcastByCodeMap: xsync.NewMapOf[protodb.CrudReqCode, *xsync.MapOf[string, []TfnCrudBroadcastHandler]](),
}

// RegisterBroadcastByCode register a broadcast to broadcastStore, willreceive all crud operation broadcast by crudCode
func (this *TcrudBroadcaster) RegisterBroadcastByCode(msgName string, crudCode protodb.CrudReqCode, fnCrudBroadcastHandler TfnCrudBroadcastHandler) {
	msgFnMaps, ok := this.fnCrudBroadcastByCodeMap.Load(crudCode)
	if !ok {
		msgFnMaps = xsync.NewMapOf[string, []TfnCrudBroadcastHandler]()
		this.fnCrudBroadcastByCodeMap.Store(crudCode, msgFnMaps)
	}

	fns, ok := msgFnMaps.Load(msgName)
	if !ok {
		fns := []TfnCrudBroadcastHandler{fnCrudBroadcastHandler}
		msgFnMaps.Store(msgName, fns)
		return
	}

	msgFnMaps.Store(msgName, appendCrudBroadcastHandler(fns, fnCrudBroadcastHandler))
}

func (this *TcrudBroadcaster) UnregisterBroadcastByCode(msgName string, crudCode protodb.CrudReqCode, fnCrudBroadcastHandler TfnCrudBroadcastHandler) {
	msgFnMaps, ok := this.fnCrudBroadcastByCodeMap.Load(crudCode)
	if !ok {
		return
	}
	fns, ok := msgFnMaps.Load(msgName)
	if !ok {
		return
	}
	for i, fn := range fns {
		if sameCrudBroadcastHandler(fn, fnCrudBroadcastHandler) {
			msgFnMaps.Store(msgName, removeCrudBroadcastHandler(fns, i))
			return
		}
	}
}

// RegisterBroadcast register a broadcast to broadcastStore, will receive all crud operation broadcast
func (this *TcrudBroadcaster) RegisterBroadcast(msgName string, fnCrudBroadcastHandler TfnCrudBroadcastHandler) {
	fns, ok := this.fnCrudBroadcastMap.Load(msgName)
	if !ok {
		fns := []TfnCrudBroadcastHandler{fnCrudBroadcastHandler}
		this.fnCrudBroadcastMap.Store(msgName, fns)
		return
	}

	this.fnCrudBroadcastMap.Store(msgName, appendCrudBroadcastHandler(fns, fnCrudBroadcastHandler))
}

func (this *TcrudBroadcaster) UnregisterBroadcast(msgName string, fnCrudBroadcastHandler TfnCrudBroadcastHandler) {
	fns, ok := this.fnCrudBroadcastMap.Load(msgName)
	if !ok {
		return
	}
	for i, fn := range fns {
		if sameCrudBroadcastHandler(fn, fnCrudBroadcastHandler) {
			this.fnCrudBroadcastMap.Store(msgName, removeCrudBroadcastHandler(fns, i))
			return
		}
	}
}

// Broadcast a crud operation
func (this *TcrudBroadcaster) Broadcast(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
	fns, _ := this.fnCrudBroadcastMap.Load(req.TableName)
	if len(fns) > 0 {
		broadcastCrudReq(fns, meta, db, req, reqMsg, respMsg)
	}
	msgMaps, ok := this.fnCrudBroadcastByCodeMap.Load(req.Code)
	if !ok {
		return
	}
	msgFns, ok := msgMaps.Load(req.TableName)
	if !ok {
		return
	}
	broadcastCrudReq(msgFns, meta, db, req, reqMsg, respMsg)
}

func broadcastCrudReq(fns []TfnCrudBroadcastHandler, meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
	for _, fn := range fns {
		fn(meta, db, req, reqMsg, respMsg)
	}
}

func appendCrudBroadcastHandler(fns []TfnCrudBroadcastHandler, fn TfnCrudBroadcastHandler) []TfnCrudBroadcastHandler {
	next := make([]TfnCrudBroadcastHandler, 0, len(fns)+1)
	next = append(next, fns...)
	next = append(next, fn)
	return next
}

func removeCrudBroadcastHandler(fns []TfnCrudBroadcastHandler, idx int) []TfnCrudBroadcastHandler {
	next := make([]TfnCrudBroadcastHandler, 0, len(fns)-1)
	next = append(next, fns[:idx]...)
	next = append(next, fns[idx+1:]...)
	return next
}

func sameCrudBroadcastHandler(left, right TfnCrudBroadcastHandler) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
