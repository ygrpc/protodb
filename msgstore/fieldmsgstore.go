package msgstore

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

var fieldMsgStore = make(map[string]TFnGetMsg)

// RegisterFieldMsg register a proto message TFnGetMsg to fieldMsgStore
// should call in init() function
func RegisterFieldMsg(msgName string, msgGetFunc TFnGetMsg) {
	if oldmsgFn, ok := fieldMsgStore[msgName]; ok {
		oldmsg := oldmsgFn(false)
		newmsg := msgGetFunc(false)
		fmt.Println("reregister protomsg to fieldMsgStore:", msgName, "old:", oldmsg.ProtoReflect().Descriptor(), "new:", newmsg.ProtoReflect().Descriptor())
	}

	fieldMsgStore[(msgName)] = msgGetFunc
}

// GetFieldMsg get a proto.Message from fieldMsgStore
// new if you need a new message, else return global static message
func GetFieldMsg(msgName string, new bool) (proto.Message, bool) {
	msgfn, ok := fieldMsgStore[msgName]
	if !ok {
		return nil, false
	}
	return msgfn(new), true
}
