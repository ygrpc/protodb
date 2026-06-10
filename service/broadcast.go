package service

import (
	"log"
	"net/http"
	"reflect"
	"runtime/debug"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

// TfnCrudBroadcastHandler handles CRUD broadcasts.
// Async broadcasts pass cloned request/response messages, but db is the
// original executor kept for API compatibility and may be transaction-bound.
type TfnCrudBroadcastHandler func(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message)

type TcrudBroadcaster struct {
	// msgName => fns
	fnCrudBroadcastMap *xsync.MapOf[string, []TfnCrudBroadcastHandler]
	// crudCode => {msgName => fns}
	fnCrudBroadcastByCodeMap *xsync.MapOf[protodb.CrudReqCode, *xsync.MapOf[string, []TfnCrudBroadcastHandler]]
}

type crudBroadcastEvent struct {
	meta    http.Header
	db      sqldb.DB
	req     *protodb.CrudReq
	reqMsg  proto.Message
	respMsg proto.Message
}

var GlobalCrudBroadcaster *TcrudBroadcaster = &TcrudBroadcaster{
	fnCrudBroadcastMap:       xsync.NewMapOf[string, []TfnCrudBroadcastHandler](),
	fnCrudBroadcastByCodeMap: xsync.NewMapOf[protodb.CrudReqCode, *xsync.MapOf[string, []TfnCrudBroadcastHandler]](),
}

// RegisterBroadcastByCode registers a handler for one table and CRUD code.
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

// UnregisterBroadcastByCode removes one handler registered for a table and CRUD code.
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

// RegisterBroadcast registers a handler for all CRUD codes on one table.
func (this *TcrudBroadcaster) RegisterBroadcast(msgName string, fnCrudBroadcastHandler TfnCrudBroadcastHandler) {
	fns, ok := this.fnCrudBroadcastMap.Load(msgName)
	if !ok {
		fns := []TfnCrudBroadcastHandler{fnCrudBroadcastHandler}
		this.fnCrudBroadcastMap.Store(msgName, fns)
		return
	}

	this.fnCrudBroadcastMap.Store(msgName, appendCrudBroadcastHandler(fns, fnCrudBroadcastHandler))
}

// UnregisterBroadcast removes one handler registered for all CRUD codes on a table.
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

// Broadcast runs matching handlers synchronously and isolates handler panics.
func (this *TcrudBroadcaster) Broadcast(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
	fns := this.crudBroadcastHandlers(req)
	broadcastCrudReq(fns, meta, db, req, reqMsg, respMsg)
}

// BroadcastAsync snapshots mutable event data before invoking handlers in a goroutine.
func (this *TcrudBroadcaster) BroadcastAsync(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
	fns := this.crudBroadcastHandlers(req)
	if len(fns) == 0 {
		return
	}
	event := snapshotCrudBroadcastEvent(meta, db, req, reqMsg, respMsg)
	go broadcastCrudReq(fns, event.meta, event.db, event.req, event.reqMsg, event.respMsg)
}

// crudBroadcastHandlers returns a stable handler list for the request table and code.
func (this *TcrudBroadcaster) crudBroadcastHandlers(req *protodb.CrudReq) []TfnCrudBroadcastHandler {
	if req == nil {
		return nil
	}

	fns := make([]TfnCrudBroadcastHandler, 0)
	if msgFns, ok := this.fnCrudBroadcastMap.Load(req.TableName); ok {
		fns = append(fns, msgFns...)
	}
	msgMaps, ok := this.fnCrudBroadcastByCodeMap.Load(req.Code)
	if !ok {
		return fns
	}
	msgFns, ok := msgMaps.Load(req.TableName)
	if !ok {
		return fns
	}
	return append(fns, msgFns...)
}

// broadcastCrudReq invokes handlers one by one so one panic cannot stop later handlers.
func broadcastCrudReq(fns []TfnCrudBroadcastHandler, meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
	for _, fn := range fns {
		callCrudBroadcastHandler(fn, meta, db, req, reqMsg, respMsg)
	}
}

// callCrudBroadcastHandler isolates handler panics and continues later handlers.
func callCrudBroadcastHandler(fn TfnCrudBroadcastHandler, meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
	if fn == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("protodb: crud broadcast handler panic: %v\n%s", recovered, debug.Stack())
		}
	}()

	fn(meta, db, req, reqMsg, respMsg)
}

// snapshotCrudBroadcastEvent deep-copies mutable event data before async delivery.
func snapshotCrudBroadcastEvent(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) crudBroadcastEvent {
	return crudBroadcastEvent{
		meta:    cloneHTTPHeader(meta),
		db:      db,
		req:     cloneCrudReq(req),
		reqMsg:  cloneProtoMessage(reqMsg),
		respMsg: cloneProtoMessage(respMsg),
	}
}

// cloneHTTPHeader copies the header map and value slices for async handlers.
func cloneHTTPHeader(meta http.Header) http.Header {
	if meta == nil {
		return nil
	}
	return meta.Clone()
}

// cloneCrudReq deep-copies the CRUD request, including byte slices and repeated fields.
func cloneCrudReq(req *protodb.CrudReq) *protodb.CrudReq {
	if req == nil {
		return nil
	}
	return proto.Clone(req).(*protodb.CrudReq)
}

// cloneProtoMessage deep-copies arbitrary protobuf messages when present.
func cloneProtoMessage(msg proto.Message) proto.Message {
	if msg == nil {
		return nil
	}
	return proto.Clone(msg)
}

// appendCrudBroadcastHandler returns a new slice to keep published handler lists immutable.
func appendCrudBroadcastHandler(fns []TfnCrudBroadcastHandler, fn TfnCrudBroadcastHandler) []TfnCrudBroadcastHandler {
	next := make([]TfnCrudBroadcastHandler, 0, len(fns)+1)
	next = append(next, fns...)
	next = append(next, fn)
	return next
}

// removeCrudBroadcastHandler returns a new slice to avoid mutating readers' snapshots.
func removeCrudBroadcastHandler(fns []TfnCrudBroadcastHandler, idx int) []TfnCrudBroadcastHandler {
	next := make([]TfnCrudBroadcastHandler, 0, len(fns)-1)
	next = append(next, fns[:idx]...)
	next = append(next, fns[idx+1:]...)
	return next
}

// sameCrudBroadcastHandler compares function identity because functions are not comparable.
func sameCrudBroadcastHandler(left, right TfnCrudBroadcastHandler) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
