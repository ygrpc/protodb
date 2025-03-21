package crud

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func MsgUnmarshal(msg proto.Message, msgBytes []byte, msgFormat int32) (err error) {
	switch msgFormat {
	case 0:
		err = proto.Unmarshal(msgBytes, msg)
		if err != nil {
			return fmt.Errorf("unmarshal msg %s err: %w", msg.ProtoReflect().Descriptor().FullName(), err)
		}
		return nil
	case 1:
		err = protojson.Unmarshal(msgBytes, msg)
		if err != nil {
			return fmt.Errorf("unmarshal msg %s err: %w", msg.ProtoReflect().Descriptor().FullName(), err)
		}
		return nil
	default:
		return fmt.Errorf("invalid msg format: %d", msgFormat)
	}
}

func MsgMarshal(msg proto.Message, msgFormat int32) (msgBytes []byte, err error) {
	switch msgFormat {
	case 0:
		msgBytes, err = proto.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("marshal msg %s err: %w", msg.ProtoReflect().Descriptor().FullName(), err)
		}
		return msgBytes, nil
	case 1:
		msgBytes, err = protojson.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("marshal msg %s err: %w", msg.ProtoReflect().Descriptor().FullName(), err)
		}
		return msgBytes, nil
	default:
		return nil, fmt.Errorf("invalid msg format: %d", msgFormat)
	}
}
