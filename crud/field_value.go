package crud

import (
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func getSQLFieldValue(msg proto.Message, fieldDesc protoreflect.FieldDescriptor) (any, error) {
	fieldName := string(fieldDesc.Name())
	if fieldDesc.IsMap() || fieldDesc.IsList() || fieldDesc.Kind() == protoreflect.MessageKind {
		return pdbutil.GetField(msg, fieldName)
	}
	if fieldDesc.HasPresence() {
		return pdbutil.GetField(msg, fieldName)
	}

	pm := msg.ProtoReflect()
	val := pm.Get(fieldDesc)
	switch fieldDesc.Kind() {
	case protoreflect.BoolKind:
		return val.Bool(), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(val.Int()), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return val.Int(), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return uint32(val.Uint()), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return val.Uint(), nil
	case protoreflect.FloatKind:
		return float32(val.Float()), nil
	case protoreflect.DoubleKind:
		return val.Float(), nil
	case protoreflect.StringKind:
		return val.String(), nil
	case protoreflect.BytesKind:
		return val.Bytes(), nil
	case protoreflect.EnumKind:
		// Preserve generated enum values and their String behavior for existing zero/default checks.
		if goVal, err := pdbutil.GetField(msg, fieldName); err == nil {
			return goVal, nil
		}
		return int32(val.Enum()), nil
	default:
		return pdbutil.GetField(msg, fieldName)
	}
}
