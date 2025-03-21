package msgstore

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

// TFnGetMsg get a proto.Message from msgStore
// new if need a new message, else return global static message
type TFnGetMsg = func(new bool) proto.Message

var msgStore = make(map[string]TFnGetMsg)

// RegisterMsg register a proto message TFnGetMsg to msgStore
// should call in init() function
func RegisterMsg(msgName string, msgGetFunc TFnGetMsg) {
	if oldmsgFn, ok := msgStore[msgName]; ok {
		oldmsg := oldmsgFn(false)
		newmsg := msgGetFunc(false)
		fmt.Println("reregister protomsg to msgStore:", msgName, "old:", oldmsg.ProtoReflect().Descriptor(), "new:", newmsg.ProtoReflect().Descriptor())
	}

	msgStore[(msgName)] = msgGetFunc
}

// GetMsg get a proto.Message from msgStore
// new if need a new message, else return global static message
func GetMsg(msgName string, new bool) (proto.Message, bool) {
	msgfn, ok := msgStore[msgName]
	if !ok {
		return nil, false
	}
	return msgfn(new), true
}
