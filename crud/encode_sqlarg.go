package crud

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func EncodeSQLArg(fieldDesc protoreflect.FieldDescriptor, dialect sqldb.TDBDialect, goValue any) (any, error) {
	if goValue == nil {
		return nil, nil
	}
	if ns, ok := goValue.(sql.NullString); ok {
		if !ns.Valid {
			return ns, nil
		}
	}

	if fieldDesc.IsList() {
		v := reflect.ValueOf(goValue)
		if v.IsValid() && v.Kind() == reflect.Slice && v.IsNil() {
			goValue = reflect.MakeSlice(v.Type(), 0, 0).Interface()
		}

		// For Postgres, our DDL maps uint32/uint64 to bigint, so convert to []int64 for arrays.
		if dialect == sqldb.Postgres {
			switch fieldDesc.Kind() {
			case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
				if s, ok := goValue.([]uint32); ok {
					out := make([]int64, 0, len(s))
					for _, x := range s {
						out = append(out, int64(x))
					}
					goValue = out
				}
			case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
				if s, ok := goValue.([]uint64); ok {
					out := make([]int64, 0, len(s))
					for _, x := range s {
						out = append(out, int64(x))
					}
					goValue = out
				}
			}
		}

		if fieldDesc.Kind() == protoreflect.MessageKind {
			val := reflect.ValueOf(goValue)
			if !val.IsValid() || val.Kind() != reflect.Slice {
				return nil, fmt.Errorf("repeated message value must be slice, got %T", goValue)
			}
			raws := make([]json.RawMessage, 0, val.Len())
			for i := 0; i < val.Len(); i++ {
				e := val.Index(i)
				if e.Kind() == reflect.Pointer && e.IsNil() {
					continue
				}
				pm, ok := e.Interface().(proto.Message)
				if !ok {
					return nil, fmt.Errorf("repeated message elem must be proto.Message, got %T", val.Index(i).Interface())
				}
				b, err := protojson.Marshal(pm)
				if err != nil {
					return nil, err
				}
				raws = append(raws, json.RawMessage(b))
			}
			b, err := json.Marshal(raws)
			if err != nil {
				return nil, err
			}
			return string(b), nil
		}

		if dialect == sqldb.SQLite {
			b, err := json.Marshal(goValue)
			if err != nil {
				return nil, err
			}
			return string(b), nil
		}
		return goValue, nil
	}

	if fieldDesc.Kind() == protoreflect.MessageKind {
		pm, ok := goValue.(proto.Message)
		if !ok {
			return nil, fmt.Errorf("message field expects proto.Message, got %T", goValue)
		}
		b, err := protojson.Marshal(pm)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}

	return goValue, nil
}
