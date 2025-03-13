package msgstore

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

var msgStore = make(map[string]proto.Message)

// RegisterMsg register a proto message to msgStore
// should call in init() function
func RegisterMsg(msg proto.Message) {
	desc := msg.ProtoReflect().Descriptor()
	msgName := string(desc.Name())
	msgFullName := string(desc.FullName())

	if oldmsg, ok := msgStore[(msgName)]; ok {
		fmt.Println("reregister protomsg to msgStore:", msgName, "old:", oldmsg.ProtoReflect().Descriptor(), "new:", desc)
	}

	msgStore[(msgName)] = msg
	msgStore[(msgFullName)] = msg
}

// GetMsg get a proto message from msgStore
func GetMsg(msgName string) (proto.Message, bool) {
	msg, ok := msgStore[msgName]
	return msg, ok
}
