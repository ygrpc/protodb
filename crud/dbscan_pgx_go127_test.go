//go:build go1.27

package crud

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestPGXColumnReceivers(t *testing.T) {
	m := pgtype.NewMap()
	for _, tc := range []struct {
		field string
		oid   uint32
		value any
	}{
		{"id", pgtype.Int8OID, int64(123456)},
		{"count", pgtype.Int4OID, int32(-123)},
		{"flags", pgtype.Int2OID, int16(42)},
		{"hash", pgtype.Int8OID, int64(123456)},
		{"name", pgtype.TextOID, "世界"},
		{"active", pgtype.BoolOID, true},
		{"score", pgtype.Float4OID, float32(1.25)},
		{"amount", pgtype.Float8OID, float64(2.5)},
		{"data", pgtype.ByteaOID, []byte("bytes")},
	} {
		for _, format := range []int16{pgtype.TextFormatCode, pgtype.BinaryFormatCode} {
			t.Run(tc.field+map[int16]string{0: "/text", 1: "/binary"}[format], func(t *testing.T) {
				fd := benchMsgDesc.Fields().ByName(protoreflect.Name(tc.field))
				got, want := dynamicpb.NewMessage(benchMsgDesc), dynamicpb.NewMessage(benchMsgDesc)
				r := newPGXFieldReceiver(&protoFieldReceiver{msg: got, fd: fd})
				if _, ok := any(r).(sql.Scanner); ok {
					t.Fatal("native receiver must not implement sql.Scanner")
				}
				src, err := m.Encode(tc.oid, format, tc.value, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := m.Scan(tc.oid, format, src, r); err != nil {
					t.Fatal(err)
				}
				typ, _ := m.TypeForOID(tc.oid)
				v, err := typ.Codec.DecodeDatabaseSQLValue(m, tc.oid, format, src)
				if err != nil {
					t.Fatal(err)
				}
				if err := (&protoFieldReceiver{msg: want, fd: fd}).Scan(v); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(got, want) {
					t.Fatalf("got %v, want %v", got, want)
				}
				if err := m.Scan(tc.oid, format, nil, r); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(got, want) {
					t.Fatal("NULL changed existing value")
				}
				// Mutating the source must not mutate retained string/bytes fields.
				clear(src)
				if !proto.Equal(got, want) {
					t.Fatal("retained driver buffer")
				}
				next := dynamicpb.NewMessage(benchMsgDesc)
				r.bind(next)
				if err := m.Scan(tc.oid, format, nil, r); err != nil {
					t.Fatal(err)
				}
				if next.Has(fd) {
					t.Fatal("NULL populated a fresh message")
				}
			})
		}
	}
}

const pgxArrayScanQuery = `SELECT ARRAY(SELECT 'value-' || n FROM generate_series(1,64) AS n) AS "Comment" FROM generate_series(1,1000)`

func TestPGXGeneratedStringArrays(t *testing.T) {
	for _, tc := range []struct {
		msg   proto.Message
		field protoreflect.Name
	}{
		{&protodb.PDBField{}, "Comment"},
		{&protodb.PDBField{}, "SQLAppend"},
		{&protodb.PDBMsg{}, "SQLMigrate"},
	} {
		t.Run(string(tc.field), func(t *testing.T) {
			fd := tc.msg.ProtoReflect().Descriptor().Fields().ByName(tc.field)
			r := newPGXStringArrayReceiver(&protoFieldReceiver{msg: tc.msg, fd: fd})
			if r == nil {
				t.Fatal("expected generated receiver")
			}
			m := pgtype.NewMap()
			for _, format := range []int16{pgtype.TextFormatCode, pgtype.BinaryFormatCode} {
				msg := tc.msg.ProtoReflect().New().Interface()
				r.bind(msg)
				src, err := m.Encode(pgtype.TextArrayOID, format, []string{"a", " b "}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := m.Scan(pgtype.TextArrayOID, format, src, r); err != nil {
					t.Fatal(err)
				}
				if err := r.finish(); err != nil {
					t.Fatal(err)
				}
				clear(src)
				list := msg.ProtoReflect().Get(fd).List()
				if list.Len() != 2 || list.Get(1).String() != " b " {
					t.Fatalf("got %v", msg)
				}
				r.bind(tc.msg.ProtoReflect().New().Interface())
				if err := m.Scan(pgtype.TextArrayOID, format, nil, r); err != nil {
					t.Fatal(err)
				}
				if err := r.finish(); err != nil {
					t.Fatal(err)
				}
				if list.Len() != 2 || list.Get(0).String() != "a" {
					t.Fatal("previous message changed")
				}
			}
		})
	}
}

func TestPGXNativeArrayCodecs(t *testing.T) {
	file := protodesc.ToFileDescriptorProto(benchMsgDesc.ParentFile())
	for _, fd := range file.MessageType[0].Field {
		fd.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	}
	desc, err := protodesc.NewFile(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	md := desc.Messages().Get(0)
	m := pgtype.NewMap()
	for _, tc := range []struct {
		field  string
		oid    uint32
		values any
		want   []any
	}{
		{"id", pgtype.Int8ArrayOID, []int64{123456, -1}, []any{int64(123456), int64(-1)}},
		{"count", pgtype.Int4ArrayOID, []int32{123, -1}, []any{int64(123), int64(-1)}},
		{"flags", pgtype.Int2ArrayOID, []int16{42, 0}, []any{int64(42), int64(0)}},
		{"hash", pgtype.Int8ArrayOID, []int64{123456, 0}, []any{int64(123456), int64(0)}},
		{"score", pgtype.Float4ArrayOID, []float32{1.25, 0}, []any{float64(1.25), float64(0)}},
		{"amount", pgtype.Float8ArrayOID, []float64{2.5, 0}, []any{float64(2.5), float64(0)}},
		{"active", pgtype.BoolArrayOID, []bool{true, false}, []any{true, false}},
		{"name", pgtype.TextArrayOID, []string{"世界", "a,b"}, []any{"世界", "a,b"}},
		{"data", pgtype.ByteaArrayOID, [][]byte{[]byte("abc"), nil}, []any{[]byte("abc"), []byte(nil)}},
	} {
		for _, format := range []int16{pgtype.TextFormatCode, pgtype.BinaryFormatCode} {
			t.Run(tc.field+map[int16]string{0: "/text", 1: "/binary"}[format], func(t *testing.T) {
				msg := dynamicpb.NewMessage(md)
				fd := md.Fields().ByName(protoreflect.Name(tc.field))
				base := &pgxFieldReceiver{receiver: &protoFieldReceiver{msg: msg, fd: fd}}
				r := &pgxArrayReceiver{base: base, element: newPGXElementReceiver(base)}
				src, err := m.Encode(tc.oid, format, tc.values, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := m.Scan(tc.oid, format, src, r); err != nil {
					t.Fatal(err)
				}
				list := msg.Get(fd).List()
				if list.Len() != len(tc.want) {
					t.Fatalf("length=%d", list.Len())
				}
				want := dynamicpb.NewMessage(md)
				for _, v := range tc.want {
					pv, err := scalarElemToProtoreflectValue(fd.Kind(), v)
					if err != nil {
						t.Fatal(err)
					}
					want.Mutable(fd).List().Append(pv)
				}
				if !proto.Equal(msg, want) {
					t.Fatalf("got %v, want %v", msg, want)
				}
				clear(src)
				if !proto.Equal(msg, want) {
					t.Fatal("retained driver buffer")
				}
				if err := m.Scan(tc.oid, format, nil, r); err != nil {
					t.Fatal(err)
				}
				if msg.Get(fd).List().Len() != 0 {
					t.Fatal("NULL did not clear list")
				}
				if err := r.SetDimensions([]pgtype.ArrayDimension{{Length: 2}, {Length: 2}}); err == nil {
					t.Fatal("accepted multidimensional array")
				}
			})
		}
	}
}

func TestPGXArrayRowsColumnScanner(t *testing.T) {
	for _, mode := range []pgx.QueryExecMode{pgx.QueryExecModeCacheStatement, pgx.QueryExecModeSimpleProtocol} {
		t.Run(mode.String(), func(t *testing.T) {
			db := pgxScanTestDB(t, mode)
			rows, err := QueryProtoRows(db, `SELECT v AS "Comment" FROM (VALUES
			 (ARRAY[' a ', 'NULL', NULL, '', 'a,b', '"quoted"', E'a\\b']::text[]),
			 ('{}'::text[]), (NULL::text[]), (ARRAY['last']::text[])) AS t(v)`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			prototype := &protodb.PDBField{}
			s, err := NewDbRowScannerForDB(db, rows, prototype, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := s.rowVals[0].(*pgxStringArrayReceiver); !ok {
				t.Fatal("native array receiver not selected")
			}
			want := [][]string{{" a ", "NULL", "", "", "a,b", "\"quoted\"", "a\\b"}, {}, {}, {"last"}}
			var first *protodb.PDBField
			for i := 0; rows.Next(); i++ {
				if i >= len(want) {
					t.Fatal("too many rows")
				}
				msg := &protodb.PDBField{Comment: []string{"stale"}}
				if err := s.Scan(rows, msg); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(msg, &protodb.PDBField{Comment: want[i]}) {
					t.Fatalf("row %d: got %q want %q", i, msg.Comment, want[i])
				}
				if i == 0 {
					first = msg
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if first == nil || first.Comment[0] != " a " {
				t.Fatal("first row corrupted")
			}
			// The explicit constructor also works on prepared transaction rows.
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			stmt, err := tx.Prepare(`SELECT ARRAY['prepared']::text[] AS "Comment"`)
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()
			prepared, err := stmt.Query()
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			msg := &protodb.PDBField{}
			s, err = NewPGXDbRowScanner(prepared, msg, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !prepared.Next() {
				t.Fatal("no prepared row")
			}
			if err := s.Scan(prepared, msg); err != nil {
				t.Fatal(err)
			}
			if len(msg.Comment) != 1 || msg.Comment[0] != "prepared" {
				t.Fatalf("got %v", msg)
			}
		})
	}
}

func TestPGXArrayFallbackAndPair(t *testing.T) {
	db := pgxScanTestDB(t, pgx.QueryExecModeCacheStatement)
	for _, query := range []string{
		`SELECT '["a","b"]'::jsonb AS "Comment", ARRAY[1,2]::int4[] AS ignored`,
		`SELECT ARRAY['a','b']::text[] AS "Comment", ARRAY[1,2]::int4[] AS ignored`,
	} {
		rows, err := QueryProtoRows(db, query)
		if err != nil {
			t.Fatal(err)
		}
		msg := &protodb.PDBField{}
		s, err := NewDbRowScannerForDB(db, rows, msg, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			t.Fatal("no row")
		}
		if err := s.Scan(rows, msg); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(msg, &protodb.PDBField{Comment: []string{"a", "b"}}) {
			t.Fatalf("got %v", msg)
		}
		rows.Close()
	}
	// Mismatched protobuf kinds retain the old text conversion, including NULL.
	for _, query := range []string{
		`SELECT ARRAY['a','b']::text[] AS "Reference"`,
		`SELECT ARRAY[$1::text,'b'] AS "Reference"`,
	} {
		args := []any(nil)
		if strings.Contains(query, "$1") {
			args = []any{"a"}
		}
		rows, err := QueryProtoRows(db, query, args...)
		if err != nil {
			t.Fatal(err)
		}
		msg := &protodb.PDBField{}
		s, err := NewDbRowScannerForDB(db, rows, msg, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			t.Fatal("no fallback row")
		}
		if err := s.Scan(rows, msg); err != nil {
			t.Fatal(err)
		}
		if msg.Reference != "{a,b}" {
			t.Fatalf("reference=%q", msg.Reference)
		}
		rows.Close()
	}
	rows, err := QueryProtoRows(db, `SELECT ARRAY['old']::text[] AS "Comment", ARRAY['new']::text[] AS "Comment"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no pair row")
	}
	oldMsg, newMsg := &protodb.PDBField{}, &protodb.PDBField{}
	if err := dbScan2ProtoMsgPairForDB(db, rows, oldMsg, newMsg, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(oldMsg, &protodb.PDBField{Comment: []string{"old"}}) || !proto.Equal(newMsg, &protodb.PDBField{Comment: []string{"new"}}) {
		t.Fatalf("old=%v new=%v", oldMsg, newMsg)
	}
}

func TestPGXPairKeepsScalarValidation(t *testing.T) {
	db := pgxScanTestDB(t, pgx.QueryExecModeCacheStatement)
	rows, err := QueryProtoRows(db, `SELECT 1.25::float8 AS "SerialType", 2.5::float8 AS "SerialType"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no row")
	}
	if err := dbScan2ProtoMsgPairForDB(db, rows, &protodb.PDBField{}, &protodb.PDBField{}, nil, nil); err == nil {
		t.Fatal("pair scan must reject fractional values for integer destinations")
	}
}

func BenchmarkPGXArrayRowsColumnScanner(b *testing.B) {
	db := pgxScanTestDB(b, pgx.QueryExecModeCacheStatement)
	for _, tc := range []struct {
		name           string
		native, binary bool
	}{
		{"SQLScannerText", false, false},
		{"NativeBinary", true, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var rows *sql.Rows
				var err error
				if tc.binary {
					rows, err = QueryProtoRows(db, pgxArrayScanQuery)
				} else {
					rows, err = db.Query(pgxArrayScanQuery)
				}
				if err != nil {
					b.Fatal(err)
				}
				var s *DbRowScanner
				if tc.native {
					s, err = NewPGXDbRowScanner(rows, &protodb.PDBField{}, nil, nil)
				} else {
					s, err = NewDbRowScanner(rows, &protodb.PDBField{}, nil, nil)
				}
				if err != nil {
					b.Fatal(err)
				}
				count := 0
				for rows.Next() {
					msg := &protodb.PDBField{}
					if err := s.Scan(rows, msg); err != nil {
						b.Fatal(err)
					}
					if len(msg.Comment) != 64 || msg.Comment[63] != "value-64" {
						b.Fatal("incorrect array")
					}
					count++
				}
				if err := rows.Err(); err != nil {
					b.Fatal(err)
				}
				rows.Close()
				if count != 1000 {
					b.Fatalf("rows=%d", count)
				}
			}
		})
	}
}

func TestPGXColumnIntegerOverflow(t *testing.T) {
	for _, tc := range []struct {
		field string
		value int64
	}{
		{"count", 1 << 31}, {"count", -1<<31 - 1}, {"flags", -1}, {"flags", 1 << 32}, {"hash", -1},
	} {
		msg := dynamicpb.NewMessage(benchMsgDesc)
		fd := benchMsgDesc.Fields().ByName(protoreflect.Name(tc.field))
		r := newPGXFieldReceiver(&protoFieldReceiver{msg: msg, fd: fd}).(pgxIntReceiver)
		if err := r.ScanInt64(pgtype.Int8{Int64: tc.value, Valid: true}); err == nil {
			t.Fatalf("accepted %s=%d", tc.field, tc.value)
		}
		if msg.Has(fd) {
			t.Fatal("overflow mutated message")
		}
	}
}

func pgxScanTestDB(tb testing.TB, mode pgx.QueryExecMode) *sql.DB {
	tb.Helper()
	dsn := os.Getenv("PROTODB_SCAN_TEST_DSN")
	if dsn == "" {
		tb.Skip("set PROTODB_SCAN_TEST_DSN to run PostgreSQL scan integration tests")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		tb.Fatal(err)
	}
	config.DefaultQueryExecMode = mode
	db := stdlib.OpenDB(*config)
	tb.Cleanup(func() { db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		tb.Fatal(err)
	}
	return db
}

const pgxScanQuery = `SELECT i::bigint AS id, ('name-' || i)::text AS name,
 i % 2 = 0 AS active, 1.25::real AS score, 2.5::float8 AS amount,
 i::integer AS count, i::smallint AS flags, i::bigint AS hash,
 decode('010203', 'hex') AS data, '{}'::jsonb AS ignored
 FROM generate_series(1000, 1999) AS i`

func TestPGXRowsColumnScanner(t *testing.T) {
	for _, mode := range []pgx.QueryExecMode{pgx.QueryExecModeCacheStatement, pgx.QueryExecModeSimpleProtocol} {
		t.Run(mode.String(), func(t *testing.T) {
			db := pgxScanTestDB(t, mode)
			for _, native := range []bool{false, true} {
				rows, err := db.Query(pgxScanQuery)
				if err != nil {
					t.Fatal(err)
				}
				prototype := dynamicpb.NewMessage(benchMsgDesc)
				var scanner *DbRowScanner
				if native {
					scanner, err = NewDbRowScannerForDB(db, rows, prototype, nil, nil)
				} else {
					scanner, err = NewDbRowScanner(rows, prototype, nil, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := scanner.rowVals[0].(*protoFieldReceiver); !ok {
					t.Fatal("scalar should keep measured faster SQLScanner path")
				}
				var first *dynamicpb.Message
				count := 0
				for rows.Next() {
					msg := dynamicpb.NewMessage(benchMsgDesc)
					if err := scanner.Scan(rows, msg); err != nil {
						t.Fatal(err)
					}
					if got := msg.Get(benchMsgDesc.Fields().ByName("id")).Int(); got != int64(1000+count) {
						t.Fatalf("id=%d", got)
					}
					if count == 0 {
						first = msg
					}
					count++
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				rows.Close()
				if count != 1000 {
					t.Fatalf("rows=%d", count)
				}
				if got := first.Get(benchMsgDesc.Fields().ByName("name")).String(); got != "name-1000" {
					t.Fatalf("retained name=%q", got)
				}
				if got := first.Get(benchMsgDesc.Fields().ByName("data")).Bytes(); string(got) != "\x01\x02\x03" {
					t.Fatal("retained bytes changed")
				}
			}
		})
	}
}

// End-to-end: the same pgx driver, query, 1000 rows, and fresh message per row.
// Only the destination receivers differ; includes query/network and setup costs.
func BenchmarkPGXRowsColumnScanner(b *testing.B) {
	db := pgxScanTestDB(b, pgx.QueryExecModeCacheStatement)
	for _, native := range []bool{false, true} {
		name := "SQLScanner"
		if native {
			name = "Native"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rows, err := db.Query(pgxScanQuery)
				if err != nil {
					b.Fatal(err)
				}
				msg := dynamicpb.NewMessage(benchMsgDesc)
				var s *DbRowScanner
				if native {
					s, err = NewPGXDbRowScanner(rows, msg, nil, nil)
				} else {
					s, err = NewDbRowScanner(rows, msg, nil, nil)
				}
				if err != nil {
					b.Fatal(err)
				}
				count := 0
				for rows.Next() {
					msg = dynamicpb.NewMessage(benchMsgDesc)
					if err := s.Scan(rows, msg); err != nil {
						b.Fatal(err)
					}
					count++
				}
				if err := rows.Err(); err != nil {
					b.Fatal(err)
				}
				rows.Close()
				if count != 1000 {
					b.Fatalf("rows=%d", count)
				}
			}
		})
	}
}
