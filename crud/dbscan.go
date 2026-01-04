package crud

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/pdbutil"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// interface FieldProtoMsg
type FieldProtoMsg interface {
	FieldProtoMsg(fieldName string) (proto.Message, bool)
}

// DbScan2ProtoMsg scan db rows to proto message, the proto msg has no nested message
func DbScan2ProtoMsg(rows *sql.Rows, msg proto.Message, columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) (err error) {
	if columnNames == nil {
		columnNames, err = rows.Columns()
		if err != nil {
			return err
		}
	}

	rowVals := make([]interface{}, len(columnNames))
	for i := range rowVals {
		rowVals[i] = new(interface{})
	}
	err = rows.Scan(rowVals...)
	if err != nil {
		fmt.Println("DbScan2ProtoMsg err:", err)
		return err
	}

	if msgFieldsMap == nil {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(columnNames, msg.ProtoReflect().Descriptor().Fields(), true)
	}

	for i := 0; i < len(columnNames); i++ {
		columnName := strings.ToLower(columnNames[i])
		fieldDesc, ok := msgFieldsMap[columnName]
		if !ok {
			fmt.Println("DbScan2ProtoMsg field not found in msgFieldsMap :", columnName)
			fmt.Println("msgFieldsMap:", msgFieldsMap)
			continue

		}

		err = SetProtoMsgField(msg, fieldDesc, rowVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsg SetProtoMsgField err:", err)
			return err

		}
	}

	return nil
}

func Val2ProtoMsgByJson(msg proto.Message, value interface{}) error {
	if value == nil {
		return nil
	}

	var b []byte // This will hold the JSON bytes

	// 3. Use a type switch to determine the type of 'value' and get bytes
	switch v := value.(type) {
	case *any:
		anyVal := *v
		return Val2ProtoMsgByJson(msg, anyVal)
	case string:
		// If it's a string, convert it to a byte slice
		b = []byte(v)
	case []byte:
		// If it's already a byte slice, use it directly
		b = v
	case *string:
		// If it's a pointer to a string
		if v == nil {
			return nil
		}
		b = []byte(*v) // Dereference and convert
	case *[]byte:
		// If it's a pointer to a byte slice
		if v == nil {
			return nil
		}
		b = *v // Dereference
	default:
		// If it's none of the expected types
		return fmt.Errorf("unsupported type for value: %T", value)
	}

	// 4. Unmarshal the JSON bytes into the proto message
	// You can use default options or configure them:
	unmarshalOpts := protojson.UnmarshalOptions{
		DiscardUnknown: true, // Example: Ignore fields in JSON not in the proto
	}
	// return protojson.Unmarshal(b, msg) // Using default options
	return unmarshalOpts.Unmarshal(b, msg) // Using configured options
}

func SetProtoMsgField(msg proto.Message, fieldDesc protoreflect.FieldDescriptor, fieldVal interface{}) error {
	fieldName := fieldDesc.TextName()
	if fieldDesc.IsMap() {
		return setProtoMsgMapField(msg, fieldDesc, fieldVal)
	}
	if fieldDesc.IsList() {
		return setProtoMsgListField(msg, fieldDesc, fieldVal)
	}
	if fieldDesc.Kind() == protoreflect.MessageKind {
		// get filed proto msg by filedprotomsg interface
		filedProtoMsg, ok := msg.(FieldProtoMsg)
		if ok {
			fieldMsg, fieldMsgOk := filedProtoMsg.FieldProtoMsg(fieldName)
			if !fieldMsgOk {
				fieldMsgProto := fieldDesc.Message()
				return fmt.Errorf("FieldProtoMsg:can't get filed proto msg for field %s.%s", fieldMsgProto.Name(), fieldName)
			}
			err := Val2ProtoMsgByJson(fieldMsg, fieldVal)
			if err != nil {
				fieldMsgProto := fieldDesc.Message()
				return fmt.Errorf("Val2ProtoMsgByJson err:%s for field %s.%s val:%v", err.Error(), fieldMsgProto.Name(), fieldName, fieldVal)
			}
			return pdbutil.SetField(msg, fieldName, fieldMsg)
		}

		//get filed proto msg by filedmsgstore
		fieldMsgProto := fieldDesc.Message()
		fieldMsgName := string(fieldMsgProto.Name())
		fieldMsg, fieldMsgOk := msgstore.GetFieldMsg(fieldMsgName, true)
		if !fieldMsgOk {
			return fmt.Errorf("can't get filed proto msg for field %s.%s, you can register using msgstore.RegisterFieldMsg", fieldMsgProto.Name(), fieldName)
		}
		err := Val2ProtoMsgByJson(fieldMsg, fieldVal)
		if err != nil {
			return fmt.Errorf("Val2ProtoMsgByJson err:%s for field %s.%s val:%v", err.Error(), fieldMsgProto.Name(), fieldName, fieldVal)
		}
		return pdbutil.SetField(msg, fieldName, fieldMsg)

	}
	return pdbutil.SetField(msg, fieldName, fieldVal)
}

func unwrapScanVal(v any) any {
	switch x := v.(type) {
	case *any:
		if x == nil {
			return nil
		}
		return unwrapScanVal(*x)
	case *string:
		if x == nil {
			return nil
		}
		return *x
	case *[]byte:
		if x == nil {
			return nil
		}
		return *x
	default:
		return v
	}
}

func setProtoMsgMapField(msg proto.Message, fieldDesc protoreflect.FieldDescriptor, fieldVal any) error {
	v := unwrapScanVal(fieldVal)
	pm := msg.ProtoReflect()
	m := pm.Mutable(fieldDesc).Map()

	// clear existing keys
	keys := make([]protoreflect.MapKey, 0, m.Len())
	m.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
		keys = append(keys, k)
		return true
	})
	for _, k := range keys {
		m.Clear(k)
	}

	if v == nil {
		return nil
	}

	var b []byte
	switch x := v.(type) {
	case string:
		b = []byte(x)
	case []byte:
		b = x
	default:
		return fmt.Errorf("map scan expects json string/bytes, got %T", v)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}

	var raws map[string]json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return err
	}

	keyDesc := fieldDesc.MapKey()
	valDesc := fieldDesc.MapValue()
	unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}

	for ks, raw := range raws {
		mk, err := parseMapKeyFromString(keyDesc.Kind(), ks)
		if err != nil {
			return err
		}

		if valDesc.Kind() == protoreflect.MessageKind {
			elemMsg := dynamicpb.NewMessage(valDesc.Message())
			if err := unmarshalOpts.Unmarshal(raw, elemMsg); err != nil {
				return err
			}
			m.Set(mk, protoreflect.ValueOfMessage(elemMsg.ProtoReflect()))
			continue
		}

		anyVal, err := decodeJSONAny(raw)
		if err != nil {
			return err
		}
		pv, err := scalarElemToProtoreflectValue(valDesc.Kind(), anyVal)
		if err != nil {
			return err
		}
		m.Set(mk, pv)
	}

	return nil
}

func decodeJSONAny(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func parseMapKeyFromString(kind protoreflect.Kind, s string) (protoreflect.MapKey, error) {
	ss := strings.TrimSpace(s)
	switch kind {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(ss).MapKey(), nil
	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(strings.ToLower(ss))
		if err != nil {
			if ss == "1" {
				return protoreflect.ValueOfBool(true).MapKey(), nil
			}
			if ss == "0" {
				return protoreflect.ValueOfBool(false).MapKey(), nil
			}
			return protoreflect.MapKey{}, err
		}
		return protoreflect.ValueOfBool(b).MapKey(), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		i, err := strconv.ParseInt(ss, 10, 32)
		if err != nil {
			return protoreflect.MapKey{}, err
		}
		return protoreflect.ValueOfInt32(int32(i)).MapKey(), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		i, err := strconv.ParseInt(ss, 10, 64)
		if err != nil {
			return protoreflect.MapKey{}, err
		}
		return protoreflect.ValueOfInt64(i).MapKey(), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		u, err := strconv.ParseUint(ss, 10, 32)
		if err != nil {
			return protoreflect.MapKey{}, err
		}
		return protoreflect.ValueOfUint32(uint32(u)).MapKey(), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		u, err := strconv.ParseUint(ss, 10, 64)
		if err != nil {
			return protoreflect.MapKey{}, err
		}
		return protoreflect.ValueOfUint64(u).MapKey(), nil
	default:
		return protoreflect.MapKey{}, fmt.Errorf("unsupported map key kind %v", kind)
	}
}

func setProtoMsgListField(msg proto.Message, fieldDesc protoreflect.FieldDescriptor, fieldVal any) error {
	v := unwrapScanVal(fieldVal)
	pm := msg.ProtoReflect()
	list := pm.Mutable(fieldDesc).List()
	list.Truncate(0)
	if v == nil {
		return nil
	}

	if fieldDesc.Kind() == protoreflect.MessageKind {
		var b []byte
		switch x := v.(type) {
		case string:
			b = []byte(x)
		case []byte:
			b = x
		default:
			return fmt.Errorf("repeated message scan expects json string/bytes, got %T", v)
		}
		if len(b) == 0 {
			return nil
		}
		var raws []json.RawMessage
		if err := json.Unmarshal(b, &raws); err != nil {
			return err
		}
		unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
		for _, raw := range raws {
			elemMsg := dynamicpb.NewMessage(fieldDesc.Message())
			if err := unmarshalOpts.Unmarshal(raw, elemMsg); err != nil {
				return err
			}
			list.Append(protoreflect.ValueOfMessage(elemMsg.ProtoReflect()))
		}
		return nil
	}

	switch x := v.(type) {
	case string:
		return appendScalarListFromEncodedString(list, fieldDesc.Kind(), x)
	case []byte:
		return appendScalarListFromEncodedString(list, fieldDesc.Kind(), string(x))
	default:
		val := reflect.ValueOf(v)
		if val.IsValid() && val.Kind() == reflect.Slice {
			for i := 0; i < val.Len(); i++ {
				e, err := scalarElemToProtoreflectValue(fieldDesc.Kind(), val.Index(i).Interface())
				if err != nil {
					return err
				}
				list.Append(e)
			}
			return nil
		}
		return fmt.Errorf("repeated scalar scan expects slice or encoded string, got %T", v)
	}
}

func appendScalarListFromEncodedString(list protoreflect.List, kind protoreflect.Kind, s string) error {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return nil
	}
	if strings.HasPrefix(ss, "[") {
		var arr []any
		if err := json.Unmarshal([]byte(ss), &arr); err != nil {
			return err
		}
		for _, item := range arr {
			pv, err := scalarElemToProtoreflectValue(kind, item)
			if err != nil {
				return err
			}
			list.Append(pv)
		}
		return nil
	}
	if strings.HasPrefix(ss, "{") {
		elems, err := parsePGArrayLiteral(ss)
		if err != nil {
			return err
		}
		for _, item := range elems {
			pv, err := scalarElemToProtoreflectValue(kind, item)
			if err != nil {
				return err
			}
			list.Append(pv)
		}
		return nil
	}
	return fmt.Errorf("unsupported encoded list value: %q", ss)
}

func scalarElemToProtoreflectValue(kind protoreflect.Kind, v any) (protoreflect.Value, error) {
	switch kind {
	case protoreflect.BoolKind:
		switch x := v.(type) {
		case bool:
			return protoreflect.ValueOfBool(x), nil
		case int64:
			return protoreflect.ValueOfBool(x != 0), nil
		case float64:
			return protoreflect.ValueOfBool(x != 0), nil
		case string:
			b, err := strconv.ParseBool(strings.ToLower(x))
			if err != nil {
				if x == "1" {
					return protoreflect.ValueOfBool(true), nil
				}
				if x == "0" {
					return protoreflect.ValueOfBool(false), nil
				}
				return protoreflect.Value{}, err
			}
			return protoreflect.ValueOfBool(b), nil
		default:
			return protoreflect.Value{}, fmt.Errorf("bool elem unsupported type %T", v)
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		i, err := toInt64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(i)), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		i, err := toInt64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(i), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		i, err := toInt64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(i)), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		i, err := toInt64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(uint64(i)), nil
	case protoreflect.FloatKind:
		f, err := toFloat64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfFloat32(float32(f)), nil
	case protoreflect.DoubleKind:
		f, err := toFloat64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfFloat64(f), nil
	case protoreflect.StringKind:
		switch x := v.(type) {
		case string:
			return protoreflect.ValueOfString(x), nil
		default:
			return protoreflect.ValueOfString(fmt.Sprint(v)), nil
		}
	case protoreflect.BytesKind:
		switch x := v.(type) {
		case []byte:
			return protoreflect.ValueOfBytes(x), nil
		case string:
			b, err := base64.StdEncoding.DecodeString(x)
			if err != nil {
				return protoreflect.Value{}, err
			}
			return protoreflect.ValueOfBytes(b), nil
		default:
			return protoreflect.Value{}, fmt.Errorf("bytes elem unsupported type %T", v)
		}
	case protoreflect.EnumKind:
		i, err := toInt64(v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfEnum(protoreflect.EnumNumber(i)), nil
	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported list kind %v", kind)
	}
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case json.Number:
		if x == "" {
			return 0, nil
		}
		i, err := x.Int64()
		if err == nil {
			return i, nil
		}
		u, err2 := strconv.ParseUint(x.String(), 10, 64)
		if err2 == nil {
			return int64(u), nil
		}
		return 0, err
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case uint32:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case string:
		if x == "" {
			return 0, nil
		}
		i, err := strconv.ParseInt(x, 10, 64)
		if err == nil {
			return i, nil
		}
		u, err2 := strconv.ParseUint(x, 10, 64)
		if err2 == nil {
			return int64(u), nil
		}
		return 0, err
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case json.Number:
		if x == "" {
			return 0, nil
		}
		return x.Float64()
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case int64:
		return float64(x), nil
	case int:
		return float64(x), nil
	case string:
		if x == "" {
			return 0, nil
		}
		return strconv.ParseFloat(x, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func parsePGArrayLiteral(s string) ([]string, error) {
	ss := strings.TrimSpace(s)
	if len(ss) < 2 || ss[0] != '{' || ss[len(ss)-1] != '}' {
		return nil, fmt.Errorf("invalid pg array literal: %q", s)
	}
	ss = ss[1 : len(ss)-1]
	if ss == "" {
		return []string{}, nil
	}
	out := make([]string, 0)
	cur := strings.Builder{}
	inQuotes := false
	escape := false
	for i := 0; i < len(ss); i++ {
		c := ss[i]
		if escape {
			cur.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inQuotes = !inQuotes
			continue
		}
		if c == ',' && !inQuotes {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	out = append(out, cur.String())
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
		if strings.EqualFold(out[i], "NULL") {
			out[i] = ""
		}
	}
	return out, nil
}

// DbScan2ProtoMsgx2 scan db rows to proto message(oldMsg and newMsg), the proto msg has no nested message
func DbScan2ProtoMsgx2(rows *sql.Rows, oldMsg proto.Message, newMsg proto.Message, columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) (err error) {
	if columnNames == nil {
		columnNames, err = rows.Columns()
		if err != nil {
			return err
		}
	}

	rowVals := make([]interface{}, len(columnNames))
	for i := range rowVals {
		rowVals[i] = new(interface{})
	}
	err = rows.Scan(rowVals...)
	if err != nil {
		fmt.Println("DbScan2ProtoMsgx2 err:", err)
		return err
	}

	columnNamesOld := columnNames[:len(columnNames)/2]
	// columnNamesNew:=columnNames[len(columnNames)/2:]

	oldVals := rowVals[:len(rowVals)/2]
	newVals := rowVals[len(rowVals)/2:]

	if msgFieldsMap == nil {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(columnNames, oldMsg.ProtoReflect().Descriptor().Fields(), true)
	}

	for i := 0; i < len(columnNamesOld); i++ {
		columnName := strings.ToLower(columnNamesOld[i])
		fieldDesc, ok := msgFieldsMap[columnName]
		if !ok {
			fmt.Println("DbScan2ProtoMsgx2 field not found in msgFieldsMap :", columnName)
			fmt.Println("msgFieldsMap:", msgFieldsMap)
			continue

		}

		err = SetProtoMsgField(oldMsg, fieldDesc, oldVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsgx2 SetProtoMsgField err:", err)
			return err

		}

		err = SetProtoMsgField(newMsg, fieldDesc, newVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsgx2 SetProtoMsgField err:", err)
			return err

		}
	}

	return nil
}
