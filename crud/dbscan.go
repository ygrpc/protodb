package crud

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// allocScanDest 根据 protobuf 字段类型选择最优的 scan dest 类型，消除 interface{} 装箱
func allocScanDest(fd protoreflect.FieldDescriptor) any {
	if fd.IsMap() {
		return new(sql.NullString)
	}
	if fd.IsList() {
		return new(sql.NullString)
	}
	switch fd.Kind() {
	case protoreflect.StringKind:
		return new(sql.NullString)
	case protoreflect.BoolKind:
		return new(sql.NullBool)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.EnumKind:
		return new(sql.NullInt64)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return new(nullUint64)
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return new(sql.NullFloat64)
	case protoreflect.BytesKind:
		return new(nullBytes)
	case protoreflect.MessageKind:
		return new(sql.NullString)
	default:
		return new(any)
	}
}

type nullBytes struct {
	Bytes []byte
	Valid bool
}

type nullUint64 struct {
	Uint64 uint64
	Valid  bool
}

func (n *nullUint64) Scan(src any) error {
	if src == nil {
		n.Uint64 = 0
		n.Valid = false
		return nil
	}
	u, err := toUint64(src)
	if err != nil {
		return err
	}
	n.Uint64 = u
	n.Valid = true
	return nil
}

func (n *nullBytes) Scan(src any) error {
	if src == nil {
		n.Bytes = nil
		n.Valid = false
		return nil
	}
	n.Valid = true
	switch x := src.(type) {
	case []byte:
		n.Bytes = append(n.Bytes[:0], x...)
		return nil
	case string:
		n.Bytes = append(n.Bytes[:0], x...)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into nullBytes", src)
	}
}

type directNoFallbackError struct {
	err error
}

func (e directNoFallbackError) Error() string {
	return e.err.Error()
}

func noFallback(err error) error {
	return directNoFallbackError{err: err}
}

func canFallbackDirectError(err error) bool {
	var noFallbackErr directNoFallbackError
	return !errors.As(err, &noFallbackErr)
}

// setProtoMsgFieldDirect 直接通过 protoreflect API 设置字段，绕过 pdbutil.SetField 的反射开销
func setProtoMsgFieldDirect(msg proto.Message, fd protoreflect.FieldDescriptor, v any) error {
	if fd.IsMap() {
		return setProtoMsgMapField(msg, fd, v)
	}
	if fd.IsList() {
		return setProtoMsgListField(msg, fd, v)
	}
	if fd.Kind() == protoreflect.MessageKind {
		return setProtoMsgFieldMessage(msg, fd, unwrapScanVal(v))
	}

	val := unwrapScanVal(v)
	if val == nil {
		return nil
	}

	pm := msg.ProtoReflect()
	switch fd.Kind() {
	case protoreflect.BoolKind:
		switch x := val.(type) {
		case bool:
			pm.Set(fd, protoreflect.ValueOfBool(x))
		case int64:
			pm.Set(fd, protoreflect.ValueOfBool(x != 0))
		default:
			return fmt.Errorf("bool field expects bool/int64, got %T", val)
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		i, err := toInt64(val)
		if err != nil {
			return err
		}
		if i < -1<<31 || i > 1<<31-1 {
			return noFallback(fmt.Errorf("int32 field value out of range: %d", i))
		}
		pm.Set(fd, protoreflect.ValueOfInt32(int32(i)))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		i, err := toInt64(val)
		if err != nil {
			return err
		}
		pm.Set(fd, protoreflect.ValueOfInt64(i))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		i, err := toInt64(val)
		if err != nil {
			return err
		}
		if i < 0 || i > 1<<32-1 {
			return noFallback(fmt.Errorf("uint32 field value out of range: %d", i))
		}
		pm.Set(fd, protoreflect.ValueOfUint32(uint32(i)))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		u, err := toUint64(val)
		if err != nil {
			return noFallback(err)
		}
		pm.Set(fd, protoreflect.ValueOfUint64(u))
	case protoreflect.FloatKind:
		f, err := toFloat64(val)
		if err != nil {
			return err
		}
		pm.Set(fd, protoreflect.ValueOfFloat32(float32(f)))
	case protoreflect.DoubleKind:
		f, err := toFloat64(val)
		if err != nil {
			return err
		}
		pm.Set(fd, protoreflect.ValueOfFloat64(f))
	case protoreflect.StringKind:
		switch x := val.(type) {
		case string:
			pm.Set(fd, protoreflect.ValueOfString(x))
		default:
			pm.Set(fd, protoreflect.ValueOfString(fmt.Sprint(val)))
		}
	case protoreflect.BytesKind:
		switch x := val.(type) {
		case []byte:
			pm.Set(fd, protoreflect.ValueOfBytes(append([]byte(nil), x...)))
		case string:
			b, err := base64.StdEncoding.DecodeString(x)
			if err != nil {
				return err
			}
			pm.Set(fd, protoreflect.ValueOfBytes(b))
		default:
			return fmt.Errorf("bytes field expects []byte/string, got %T", val)
		}
	case protoreflect.EnumKind:
		i, err := toInt64(val)
		if err != nil {
			return err
		}
		if i < -1<<31 || i > 1<<31-1 {
			return noFallback(fmt.Errorf("enum field value out of range: %d", i))
		}
		pm.Set(fd, protoreflect.ValueOfEnum(protoreflect.EnumNumber(i)))
	default:
		return fmt.Errorf("unsupported direct field kind %v", fd.Kind())
	}
	return nil
}

// setProtoMsgFieldMessage 设置嵌套消息字段（直接路径的辅助函数）
func setProtoMsgFieldMessage(msg proto.Message, fd protoreflect.FieldDescriptor, v any) error {
	fieldName := fd.TextName()
	filedProtoMsg, ok := msg.(FieldProtoMsg)
	if ok {
		fieldMsg, fieldMsgOk := filedProtoMsg.FieldProtoMsg(fieldName)
		if !fieldMsgOk {
			fieldMsgProto := fd.Message()
			return fmt.Errorf("FieldProtoMsg:can't get filed proto msg for field %s.%s", fieldMsgProto.Name(), fieldName)
		}
		err := Val2ProtoMsgByJson(fieldMsg, v)
		if err != nil {
			fieldMsgProto := fd.Message()
			return fmt.Errorf("Val2ProtoMsgByJson err:%s for field %s.%s val:%v", err.Error(), fieldMsgProto.Name(), fieldName, v)
		}
		return pdbutil.SetField(msg, fieldName, fieldMsg)
	}

	fieldMsgProto := fd.Message()
	fieldMsgName := string(fieldMsgProto.Name())
	fieldMsg, fieldMsgOk := msgstore.GetFieldMsg(fieldMsgName, true)
	if !fieldMsgOk {
		return fmt.Errorf("can't get filed proto msg for field %s.%s, you can register using msgstore.RegisterFieldMsg", fieldMsgProto.Name(), fieldName)
	}
	err := Val2ProtoMsgByJson(fieldMsg, v)
	if err != nil {
		return fmt.Errorf("Val2ProtoMsgByJson err:%s for field %s.%s val:%v", err.Error(), fieldMsgProto.Name(), fieldName, v)
	}
	return pdbutil.SetField(msg, fieldName, fieldMsg)
}

type DbRowScanner struct {
	columnNames  []string
	msgFieldsMap map[string]protoreflect.FieldDescriptor
	rowVals      []any
}

func NewDbRowScanner(rows *sql.Rows, msg proto.Message, columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) (*DbRowScanner, error) {
	var err error
	if columnNames == nil {
		columnNames, err = rows.Columns()
		if err != nil {
			return nil, err
		}
	}
	if msgFieldsMap == nil {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(columnNames, msg.ProtoReflect().Descriptor().Fields(), true)
	}
	rowVals := make([]any, len(columnNames))
	for i := range rowVals {
		fd, ok := msgFieldsMap[strings.ToLower(columnNames[i])]
		if ok {
			rowVals[i] = allocScanDest(fd)
		} else {
			rowVals[i] = new(any)
		}
	}
	return &DbRowScanner{
		columnNames:  columnNames,
		msgFieldsMap: msgFieldsMap,
		rowVals:      rowVals,
	}, nil
}

func (s *DbRowScanner) Scan(rows *sql.Rows, msg proto.Message) error {
	err := rows.Scan(s.rowVals...)
	if err != nil {
		fmt.Println("DbRowScanner Scan err:", err)
		return err
	}
	for i := 0; i < len(s.columnNames); i++ {
		columnName := strings.ToLower(s.columnNames[i])
		fieldDesc, ok := s.msgFieldsMap[columnName]
		if !ok {
			fmt.Println("DbRowScanner field not found in msgFieldsMap :", columnName)
			fmt.Println("msgFieldsMap:", s.msgFieldsMap)
			continue
		}
		err = setProtoMsgFieldDirect(msg, fieldDesc, s.rowVals[i])
		if err != nil {
			fmt.Println("DbRowScanner setProtoMsgFieldDirect err:", err)
			if !canFallbackDirectError(err) {
				return err
			}
			err = SetProtoMsgField(msg, fieldDesc, unwrapScanVal(s.rowVals[i]))
			if err != nil {
				fmt.Println("DbRowScanner SetProtoMsgField err:", err)
				return err
			}
		}
	}
	return nil
}

// DbScan2ProtoMsg scan db rows to proto message, the proto msg has no nested message
func DbScan2ProtoMsg(rows *sql.Rows, msg proto.Message, columnNames []string, msgFieldsMap map[string]protoreflect.FieldDescriptor) (err error) {
	scanner, err := NewDbRowScanner(rows, msg, columnNames, msgFieldsMap)
	if err != nil {
		return err
	}
	return scanner.Scan(rows, msg)
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
	case *bool:
		if x == nil {
			return nil
		}
		return *x
	case *int64:
		if x == nil {
			return nil
		}
		return *x
	case *float64:
		if x == nil {
			return nil
		}
		return *x
	case *sql.NullString:
		if x == nil || !x.Valid {
			return nil
		}
		return x.String
	case sql.NullString:
		if !x.Valid {
			return nil
		}
		return x.String
	case *sql.NullBool:
		if x == nil || !x.Valid {
			return nil
		}
		return x.Bool
	case sql.NullBool:
		if !x.Valid {
			return nil
		}
		return x.Bool
	case *sql.NullInt64:
		if x == nil || !x.Valid {
			return nil
		}
		return x.Int64
	case sql.NullInt64:
		if !x.Valid {
			return nil
		}
		return x.Int64
	case *sql.NullFloat64:
		if x == nil || !x.Valid {
			return nil
		}
		return x.Float64
	case sql.NullFloat64:
		if !x.Valid {
			return nil
		}
		return x.Float64
	case *nullBytes:
		if x == nil || !x.Valid {
			return nil
		}
		return x.Bytes
	case nullBytes:
		if !x.Valid {
			return nil
		}
		return x.Bytes
	case *nullUint64:
		if x == nil || !x.Valid {
			return nil
		}
		return x.Uint64
	case nullUint64:
		if !x.Valid {
			return nil
		}
		return x.Uint64
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

func toUint64(v any) (uint64, error) {
	switch x := v.(type) {
	case json.Number:
		if x == "" {
			return 0, nil
		}
		return strconv.ParseUint(x.String(), 10, 64)
	case int:
		if x < 0 {
			return 0, fmt.Errorf("cannot convert negative int to uint64: %d", x)
		}
		return uint64(x), nil
	case int32:
		if x < 0 {
			return 0, fmt.Errorf("cannot convert negative int32 to uint64: %d", x)
		}
		return uint64(x), nil
	case int64:
		if x < 0 {
			return 0, fmt.Errorf("cannot convert negative int64 to uint64: %d", x)
		}
		return uint64(x), nil
	case uint32:
		return uint64(x), nil
	case uint64:
		return x, nil
	case []byte:
		if len(x) == 0 {
			return 0, nil
		}
		return strconv.ParseUint(string(x), 10, 64)
	case float64:
		if x < 0 {
			return 0, fmt.Errorf("cannot convert negative float64 to uint64: %v", x)
		}
		return uint64(x), nil
	case string:
		if x == "" {
			return 0, nil
		}
		return strconv.ParseUint(x, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", v)
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

	if msgFieldsMap == nil {
		msgFieldsMap = pdbutil.BuildMsgFieldsMap(columnNames, oldMsg.ProtoReflect().Descriptor().Fields(), true)
	}
	if len(columnNames)%2 != 0 {
		return fmt.Errorf("DbScan2ProtoMsgx2 expects even column count, got %d", len(columnNames))
	}

	rowVals := make([]any, len(columnNames))
	for i := range rowVals {
		columnName := strings.ToLower(columnNames[i])
		// x2 模式下列数翻倍，前半给 oldMsg，后半给 newMsg
		// 用前半部分的字段映射来决定 dest 类型
		lookupName := columnName
		if i >= len(columnNames)/2 {
			lookupName = strings.ToLower(columnNames[i-len(columnNames)/2])
		}
		fd, ok := msgFieldsMap[lookupName]
		if ok {
			rowVals[i] = allocScanDest(fd)
		} else {
			rowVals[i] = new(any)
		}
	}

	err = rows.Scan(rowVals...)
	if err != nil {
		fmt.Println("DbScan2ProtoMsgx2 err:", err)
		return err
	}

	columnNamesOld := columnNames[:len(columnNames)/2]
	oldVals := rowVals[:len(rowVals)/2]
	newVals := rowVals[len(rowVals)/2:]

	for i := 0; i < len(columnNamesOld); i++ {
		columnName := strings.ToLower(columnNamesOld[i])
		fieldDesc, ok := msgFieldsMap[columnName]
		if !ok {
			fmt.Println("DbScan2ProtoMsgx2 field not found in msgFieldsMap :", columnName)
			fmt.Println("msgFieldsMap:", msgFieldsMap)
			continue
		}

		err = setProtoMsgFieldDirect(oldMsg, fieldDesc, oldVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsgx2 setProtoMsgFieldDirect old err:", err)
			if !canFallbackDirectError(err) {
				return err
			}
			err = SetProtoMsgField(oldMsg, fieldDesc, unwrapScanVal(oldVals[i]))
			if err != nil {
				fmt.Println("DbScan2ProtoMsgx2 SetProtoMsgField old err:", err)
				return err
			}
		}

		err = setProtoMsgFieldDirect(newMsg, fieldDesc, newVals[i])
		if err != nil {
			fmt.Println("DbScan2ProtoMsgx2 setProtoMsgFieldDirect new err:", err)
			if !canFallbackDirectError(err) {
				return err
			}
			err = SetProtoMsgField(newMsg, fieldDesc, unwrapScanVal(newVals[i]))
			if err != nil {
				fmt.Println("DbScan2ProtoMsgx2 SetProtoMsgField new err:", err)
				return err
			}
		}
	}

	return nil
}
