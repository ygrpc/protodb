package crud

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// QueryProtoRows queries rows for protobuf scanning. With Go 1.27 and a known
// pgx executor, built-in PostgreSQL arrays are requested in binary format.
func QueryProtoRows(db sqldb.DB, query string, args ...any) (*sql.Rows, error) {
	return QueryProtoRowsContext(context.Background(), db, query, args...)
}

// QueryProtoRowsContext is the context-aware variant of QueryProtoRows.
func QueryProtoRowsContext(ctx context.Context, db sqldb.DB, query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(ctx, query, pgxScanQueryArgs(db, args)...)
}

// NewDbRowScannerForDB selects native pgx column scanning when the executor is
// known to use pgx. Unknown executors (including bare transactions) use the
// portable sql.Scanner path. A PostgreSQL dialect alone does not identify pgx.
func NewDbRowScannerForDB(db sqldb.DB, rows *sql.Rows, msg proto.Message, columns []string, fields map[string]protoreflect.FieldDescriptor) (*DbRowScanner, error) {
	if usesPGX(db) {
		return NewPGXDbRowScanner(rows, msg, columns, fields)
	}
	return NewDbRowScanner(rows, msg, columns, fields)
}

func usesPGX(db sqldb.DB) bool {
	for {
		if wrapped, ok := db.(*sqldb.DBWithDialect); ok {
			db = wrapped.Executor
			continue
		}
		break
	}
	if db, ok := db.(*sql.DB); ok {
		if _, ok := db.Driver().(*stdlib.Driver); ok {
			return true
		}
	}
	return false
}

func dbScan2ProtoMsgForDB(db sqldb.DB, rows *sql.Rows, msg proto.Message, columns []string, fields map[string]protoreflect.FieldDescriptor) error {
	s, err := NewDbRowScannerForDB(db, rows, msg, columns, fields)
	if err != nil {
		return err
	}
	return s.Scan(rows, msg)
}

func dbScan2ProtoMsgPairForDB(db sqldb.DB, rows *sql.Rows, oldMsg, newMsg proto.Message, columns []string, fields map[string]protoreflect.FieldDescriptor) error {
	if !pgxNativeScanEnabled || !usesPGX(db) {
		return DbScan2ProtoMsgx2(rows, oldMsg, newMsg, columns, fields)
	}
	s, err := NewDbRowPairScanner(rows, oldMsg, columns, fields)
	if err != nil {
		return err
	}
	for i, fd := range s.fieldDescs {
		if fd == nil {
			continue
		}
		s.rowVals[i] = &protoFieldReceiver{msg: oldMsg, fd: fd, validationDest: s.rowVals[i]}
		s.rowVals[i+s.halfColumnCount] = &protoFieldReceiver{msg: newMsg, fd: fd, validationDest: s.rowVals[i+s.halfColumnCount]}
	}
	combined := &DbRowScanner{rowVals: s.rowVals}
	if err := enablePGXColumnScanning(rows, combined); err != nil {
		return err
	}
	return combined.Scan(rows, nil)
}

// NewPGXDbRowScanner enables native column receivers for rows returned by pgx
// stdlib, including transactions and prepared statements. Do not use it with
// other SQL drivers. On Go before 1.27 it uses the portable scanner.
func NewPGXDbRowScanner(rows *sql.Rows, msg proto.Message, columns []string, fields map[string]protoreflect.FieldDescriptor) (*DbRowScanner, error) {
	s, err := NewDbRowScanner(rows, msg, columns, fields)
	if err != nil {
		return nil, err
	}
	if err := enablePGXColumnScanning(rows, s); err != nil {
		return nil, err
	}
	return s, nil
}
