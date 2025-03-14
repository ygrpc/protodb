package msgstore

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

type TFnGetMsg = func(msgName string) proto.Message

var msgStore = make(map[string]TFnGetMsg)

// RegisterMsg register a proto message TFnGetMsg to msgStore
// should call in init() function
func RegisterMsg(msgName string, msgGetFunc TFnGetMsg) {
	if oldmsgFn, ok := msgStore[msgName]; ok {
		oldmsg := oldmsgFn(msgName)
		newmsg := msgGetFunc(msgName)
		fmt.Println("reregister protomsg to msgStore:", msgName, "old:", oldmsg.ProtoReflect().Descriptor(), "new:", newmsg.ProtoReflect().Descriptor())
	}

	msgStore[(msgName)] = msgGetFunc
}

// GetMsg get a proto.Message from msgStore
func GetMsg(msgName string) (proto.Message, bool) {
	msgfn, ok := msgStore[msgName]
	if !ok {
		return nil, false
	}
	return msgfn(msgName), true
}
