package msgstore

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
)

var (
	fieldMsgStoreMu sync.RWMutex
	fieldMsgStore   = make(map[string]TFnGetMsg)
)

// RegisterFieldMsg register a proto message TFnGetMsg to fieldMsgStore
// should call in init() function
func RegisterFieldMsg(msgName string, msgGetFunc TFnGetMsg) {
	fieldMsgStoreMu.RLock()
	oldmsgFn, ok := fieldMsgStore[msgName]
	fieldMsgStoreMu.RUnlock()
	if ok {
		oldmsg := oldmsgFn(false)
		newmsg := msgGetFunc(false)
		fmt.Println("reregister protomsg to fieldMsgStore:", msgName, "old:", oldmsg.ProtoReflect().Descriptor(), "new:", newmsg.ProtoReflect().Descriptor())
	}

	fieldMsgStoreMu.Lock()
	fieldMsgStore[(msgName)] = msgGetFunc
	fieldMsgStoreMu.Unlock()
}

// GetFieldMsg get a proto.Message from fieldMsgStore
// new if you need a new message, else return global static message
func GetFieldMsg(msgName string, new bool) (proto.Message, bool) {
	fieldMsgStoreMu.RLock()
	msgfn, ok := fieldMsgStore[msgName]
	fieldMsgStoreMu.RUnlock()
	if !ok {
		return nil, false
	}
	return msgfn(new), true
}
