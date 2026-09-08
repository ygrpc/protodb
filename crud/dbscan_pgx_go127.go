//go:build go1.27

package crud

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var _ driver.RowsColumnScanner = (*stdlib.Rows)(nil)

const pgxNativeScanEnabled = true

// Preserve pgx stdlib's intrinsic binary formats and add supported array OIDs.
// A caller-provided pgx query option appears later and can override this map.
var pgxProtoResultFormats = pgx.QueryResultFormatsByOID{
	pgtype.BoolOID: 1, pgtype.ByteaOID: 1, pgtype.CIDOID: 1, pgtype.DateOID: 1,
	pgtype.Float4OID: 1, pgtype.Float8OID: 1, pgtype.Int2OID: 1, pgtype.Int4OID: 1,
	pgtype.Int8OID: 1, pgtype.OIDOID: 1, pgtype.TimestampOID: 1,
	pgtype.TimestamptzOID: 1, pgtype.XIDOID: 1,
	pgtype.BoolArrayOID: 1, pgtype.ByteaArrayOID: 1, pgtype.Float4ArrayOID: 1,
	pgtype.Float8ArrayOID: 1, pgtype.Int2ArrayOID: 1, pgtype.Int4ArrayOID: 1,
	pgtype.Int8ArrayOID: 1, pgtype.TextArrayOID: 1, pgtype.VarcharArrayOID: 1,
	pgtype.BPCharArrayOID: 1,
}

func pgxScanQueryArgs(db sqldb.DB, args []any) []any {
	if !usesPGX(db) {
		return args
	}
	result := make([]any, len(args)+1)
	result[0] = pgxProtoResultFormats
	copy(result[1:], args)
	return result
}

// Deliberately does not implement sql.Scanner: pgx routes sql.Scanner through
// driver.Value and ConvertAssign even when RowsColumnScanner is available.
type pgxFieldReceiver struct {
	receiver *protoFieldReceiver
	list     protoreflect.List
}

type pgxIntReceiver struct{ *pgxFieldReceiver }
type pgxFloatReceiver struct{ *pgxFieldReceiver }
type pgxBoolReceiver struct{ *pgxFieldReceiver }
type pgxTextReceiver struct{ *pgxFieldReceiver }
type pgxBytesReceiver struct{ *pgxFieldReceiver }

func newPGXFieldReceiver(r *protoFieldReceiver) interface{ bind(proto.Message) } {
	base := &pgxFieldReceiver{receiver: r}
	return newPGXElementReceiver(base)
}

func newPGXElementReceiver(base *pgxFieldReceiver) interface{ bind(proto.Message) } {
	switch base.receiver.fd.Kind() {
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return pgxFloatReceiver{base}
	case protoreflect.BoolKind:
		return pgxBoolReceiver{base}
	case protoreflect.StringKind:
		return pgxTextReceiver{base}
	case protoreflect.BytesKind:
		return pgxBytesReceiver{base}
	default:
		return pgxIntReceiver{base}
	}
}

func (r *pgxFieldReceiver) bind(msg proto.Message) { r.receiver.msg = msg }
func (r *pgxFieldReceiver) set(v protoreflect.Value) {
	if r.list != nil {
		r.list.Append(v)
	} else {
		r.receiver.msg.ProtoReflect().Set(r.receiver.fd, v)
	}
}

func (r pgxIntReceiver) ScanInt64(v pgtype.Int8) error {
	if !v.Valid && !r.receiver.fd.IsList() {
		return nil
	}
	switch r.receiver.fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.EnumKind:
		if v.Int64 < -1<<31 || v.Int64 > 1<<31-1 {
			return fmt.Errorf("int32 field value out of range: %d", v.Int64)
		}
		if r.receiver.fd.Kind() == protoreflect.EnumKind {
			r.set(protoreflect.ValueOfEnum(protoreflect.EnumNumber(v.Int64)))
		} else {
			r.set(protoreflect.ValueOfInt32(int32(v.Int64)))
		}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		if v.Int64 < 0 || v.Int64 > 1<<32-1 {
			return fmt.Errorf("uint32 field value out of range: %d", v.Int64)
		}
		r.set(protoreflect.ValueOfUint32(uint32(v.Int64)))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if v.Int64 < 0 {
			return fmt.Errorf("cannot convert negative int64 to uint64: %d", v.Int64)
		}
		r.set(protoreflect.ValueOfUint64(uint64(v.Int64)))
	default:
		r.set(protoreflect.ValueOfInt64(v.Int64))
	}
	return nil
}

func (r pgxFloatReceiver) ScanFloat64(v pgtype.Float8) error {
	if v.Valid || r.receiver.fd.IsList() {
		if r.receiver.fd.Kind() == protoreflect.FloatKind {
			r.set(protoreflect.ValueOfFloat32(float32(v.Float64)))
		} else {
			r.set(protoreflect.ValueOfFloat64(v.Float64))
		}
	}
	return nil
}
func (r pgxBoolReceiver) ScanBool(v pgtype.Bool) error {
	if v.Valid || r.receiver.fd.IsList() {
		r.set(protoreflect.ValueOfBool(v.Bool))
	}
	return nil
}
func (r pgxTextReceiver) ScanText(v pgtype.Text) error {
	if v.Valid || r.receiver.fd.IsList() {
		r.set(protoreflect.ValueOfString(v.String))
	}
	return nil
}
func (r pgxBytesReceiver) ScanBytes(v []byte) error {
	// Driver buffers expire on Next/Close. Keep the same ownership as Scan.
	if v != nil || r.receiver.fd.IsList() {
		r.set(protoreflect.ValueOfBytes(append([]byte(nil), v...)))
	}
	return nil
}

func nativePGXScalar(fd protoreflect.FieldDescriptor, databaseType string) bool {
	if fd.IsMap() {
		return false
	}
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind, protoreflect.EnumKind:
		return databaseType == "INT2" || databaseType == "INT4" || databaseType == "INT8"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return databaseType == "FLOAT4" || databaseType == "FLOAT8"
	case protoreflect.BoolKind:
		return databaseType == "BOOL"
	case protoreflect.StringKind:
		return databaseType == "TEXT" || databaseType == "VARCHAR" || databaseType == "BPCHAR"
	case protoreflect.BytesKind:
		return databaseType == "BYTEA"
	}
	return false
}

func enablePGXColumnScanning(rows *sql.Rows, s *DbRowScanner) error {
	// pgx 5.11's native scalar destination classification costs more than its
	// SQLScanner path. Only arrays avoid enough conversion work to justify the
	// native path; leave scalar receivers on the portable path.
	columns, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	if len(columns) != len(s.rowVals) {
		return fmt.Errorf("pgx scanner: got %d columns, expected %d", len(columns), len(s.rowVals))
	}
	var typeMap *pgtype.Map
	for i, column := range columns {
		name := column.DatabaseTypeName()
		oid := pgxBinaryArrayOID(name)
		if oid == 0 {
			continue
		}
		if r, ok := s.rowVals[i].(*protoFieldReceiver); ok && r.fd.IsList() && len(name) > 1 && name[0] == '_' && nativePGXScalar(r.fd, name[1:]) {
			if fast := newPGXStringArrayReceiver(r); fast != nil {
				s.rowVals[i] = fast
				s.finishers = append(s.finishers, fast.finish)
				continue
			}
			base := &pgxFieldReceiver{receiver: r}
			s.rowVals[i] = &pgxArrayReceiver{base: base, element: newPGXElementReceiver(base)}
		} else if r, ok := s.rowVals[i].(*protoFieldReceiver); ok {
			// A mismatched protobuf kind still needs the historical text
			// conversion. pgx stdlib cannot convert binary arrays to *string.
			if typeMap == nil {
				typeMap = pgtype.NewMap()
			}
			fallback := &pgxFallbackArrayReceiver{receiver: r, typeMap: typeMap, oid: oid}
			s.rowVals[i] = fallback
			s.finishers = append(s.finishers, fallback.finish)
		} else {
			// Unknown columns are ignored, but must accept binary arrays too.
			s.rowVals[i] = new(pgtype.Array[any])
		}
	}
	return nil
}

// Generated open-API messages expose []string fields. Fill an owned slice and
// assign it once, avoiding reflect.Append inside protoreflect.List per element.
// Dynamic/opaque messages retain the general protoreflect receiver above.
type pgxStringArrayReceiver struct {
	receiver   *protoFieldReceiver
	msgType    reflect.Type
	fieldIndex int
	values     []string
	index      int
}

func newPGXStringArrayReceiver(r *protoFieldReceiver) *pgxStringArrayReceiver {
	if !r.fd.IsList() || r.fd.Kind() != protoreflect.StringKind {
		return nil
	}
	t := reflect.TypeOf(r.msg)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil
	}
	if reflect.TypeOf(r.msg.ProtoReflect().Interface()) != t {
		return nil // A wrapper exposing another message's reflection.
	}
	wantNumber := strconv.Itoa(int(r.fd.Number()))
	for i := 0; i < t.Elem().NumField(); i++ {
		f := t.Elem().Field(i)
		if f.PkgPath != "" || f.Type != reflect.TypeFor[[]string]() {
			continue
		}
		_, rest, _ := strings.Cut(f.Tag.Get("protobuf"), ",")
		number, _, _ := strings.Cut(rest, ",")
		if number == wantNumber {
			return &pgxStringArrayReceiver{receiver: r, msgType: t, fieldIndex: i}
		}
	}
	return nil
}

func (r *pgxStringArrayReceiver) bind(msg proto.Message) { r.receiver.msg = msg }
func (r *pgxStringArrayReceiver) SetDimensions(dimensions []pgtype.ArrayDimension) error {
	if len(dimensions) > 1 {
		return fmt.Errorf("protobuf repeated field %s cannot represent a multidimensional PostgreSQL array", r.receiver.fd.FullName())
	}
	r.values = nil
	if len(dimensions) == 1 {
		r.values = make([]string, int(dimensions[0].Length))
	}
	return nil
}
func (r *pgxStringArrayReceiver) ScanIndex(i int) any { r.index = i; return pgxStringArrayElement{r} }
func (r *pgxStringArrayReceiver) ScanIndexType() any  { return pgxStringArrayElement{r} }
func (r *pgxStringArrayReceiver) finish() error {
	if reflect.TypeOf(r.receiver.msg) != r.msgType {
		return fmt.Errorf("pgx array scanner requires message type %v", r.msgType)
	}
	reflect.ValueOf(r.receiver.msg).Elem().Field(r.fieldIndex).Set(reflect.ValueOf(r.values))
	return nil
}

type pgxStringArrayElement struct{ array *pgxStringArrayReceiver }

func (e pgxStringArrayElement) ScanText(v pgtype.Text) error {
	if v.Valid {
		e.array.values[e.array.index] = v.String
	}
	return nil
}

func pgxBinaryArrayOID(name string) uint32 {
	switch name {
	case "_BOOL":
		return pgtype.BoolArrayOID
	case "_BYTEA":
		return pgtype.ByteaArrayOID
	case "_FLOAT4":
		return pgtype.Float4ArrayOID
	case "_FLOAT8":
		return pgtype.Float8ArrayOID
	case "_INT2":
		return pgtype.Int2ArrayOID
	case "_INT4":
		return pgtype.Int4ArrayOID
	case "_INT8":
		return pgtype.Int8ArrayOID
	case "_TEXT":
		return pgtype.TextArrayOID
	case "_VARCHAR":
		return pgtype.VarcharArrayOID
	case "_BPCHAR":
		return pgtype.BPCharArrayOID
	}
	return 0
}

type pgxFallbackArrayReceiver struct {
	pgtype.Array[any]
	receiver *protoFieldReceiver
	typeMap  *pgtype.Map
	oid      uint32
}

func (r *pgxFallbackArrayReceiver) bind(msg proto.Message) { r.receiver.msg = msg }
func (r *pgxFallbackArrayReceiver) finish() error {
	if !r.Valid {
		return r.receiver.Scan(nil)
	}
	text, err := r.typeMap.Encode(r.oid, pgtype.TextFormatCode, r.Array, nil)
	if err != nil {
		return err
	}
	return r.receiver.Scan(string(text))
}

// A single element receiver is reused: pgx calls ScanIndex and scans that
// element synchronously before requesting the next index.
type pgxArrayReceiver struct {
	base    *pgxFieldReceiver
	element any
}

func (r *pgxArrayReceiver) bind(msg proto.Message) { r.base.receiver.msg = msg }
func (r *pgxArrayReceiver) SetDimensions(dimensions []pgtype.ArrayDimension) error {
	if len(dimensions) > 1 {
		return fmt.Errorf("protobuf repeated field %s cannot represent a multidimensional PostgreSQL array", r.base.receiver.fd.FullName())
	}
	r.base.list = r.base.receiver.msg.ProtoReflect().Mutable(r.base.receiver.fd).List()
	r.base.list.Truncate(0)
	return nil
}
func (r *pgxArrayReceiver) ScanIndex(int) any  { return r.element }
func (r *pgxArrayReceiver) ScanIndexType() any { return r.element }
