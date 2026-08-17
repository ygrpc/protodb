package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

const transactionalCrudTestTable = "transactional_crud_executor_test"

var registerTransactionalCrudTestMessageOnce sync.Once

func registerTransactionalCrudTestMessage() {
	registerTransactionalCrudTestMessageOnce.Do(func() {
		msgstore.RegisterMsg(transactionalCrudTestTable, func(bool) proto.Message {
			return &protodb.CrudResp{}
		})
	})
}

func transactionalCrudInsertRequest(t *testing.T, rowsAffected int64) *protodb.CrudReq {
	t.Helper()

	msgBytes, err := proto.Marshal(&protodb.CrudResp{RowsAffected: rowsAffected})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return &protodb.CrudReq{
		Code:       protodb.CrudReqCode_INSERT,
		ResultType: protodb.CrudResultType_DMLResult,
		TableName:  transactionalCrudTestTable,
		MsgBytes:   msgBytes,
	}
}

func beginTransactionalCrudTest(t *testing.T, permission TfnProtodbCrudPermission) (*TransactionalCrudExecutor, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	mock.ExpectBegin()
	executor, err := BeginTransactionalCrud(context.Background(), db, nil, permission)
	if err != nil {
		t.Fatalf("BeginTransactionalCrud: %v", err)
	}
	return executor, mock, db
}

func assertNoTransactionalBroadcast[T any](t *testing.T, calls <-chan T) {
	t.Helper()
	select {
	case <-calls:
		t.Fatal("unexpected CRUD broadcast")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestBeginTransactionalCrudValidatesDatabase(t *testing.T) {
	executor, err := BeginTransactionalCrud(context.Background(), nil, nil, nil)
	if err == nil {
		t.Fatal("BeginTransactionalCrud with nil database returned nil error")
	}
	if executor != nil {
		t.Fatalf("executor = %#v, want nil", executor)
	}
}

func TestBeginTransactionalCrudReturnsBeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	beginErr := errors.New("begin failed")
	mock.ExpectBegin().WillReturnError(beginErr)
	executor, err := BeginTransactionalCrud(context.Background(), db, &sql.TxOptions{ReadOnly: true}, nil)
	if err != beginErr {
		t.Fatalf("BeginTransactionalCrud error = %v, want original error %v", err, beginErr)
	}
	if executor != nil {
		t.Fatalf("executor = %#v, want nil", executor)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCrudRequestIsBroadcastWrite(t *testing.T) {
	tests := []struct {
		name string
		req  *protodb.CrudReq
		want bool
	}{
		{name: "nil", req: nil},
		{name: "insert", req: &protodb.CrudReq{Code: protodb.CrudReqCode_INSERT}, want: true},
		{name: "update", req: &protodb.CrudReq{Code: protodb.CrudReqCode_UPDATE}, want: true},
		{name: "partial update", req: &protodb.CrudReq{Code: protodb.CrudReqCode_PARTIALUPDATE}, want: true},
		{name: "delete", req: &protodb.CrudReq{Code: protodb.CrudReqCode_DELETE}, want: true},
		{name: "select one", req: &protodb.CrudReq{Code: protodb.CrudReqCode_SELECTONE}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := crudRequestIsBroadcastWrite(test.req); got != test.want {
				t.Fatalf("crudRequestIsBroadcastWrite() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestHandleCrudWithTransactionBroadcastsBeforeCommit(t *testing.T) {
	registerTransactionalCrudTestMessage()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	transaction := sqldb.NewTxWithDialect(tx, db)

	calls := make(chan struct{}, 1)
	handler := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		calls <- struct{}{}
	}
	GlobalCrudBroadcaster.RegisterBroadcast(transactionalCrudTestTable, handler)
	t.Cleanup(func() { GlobalCrudBroadcaster.UnregisterBroadcast(transactionalCrudTestTable, handler) })

	mock.ExpectExec("INSERT INTO CrudResp").WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = HandleCrud(
		context.Background(),
		nil,
		transactionalCrudInsertRequest(t, 1),
		func(http.Header, string, string, bool) (sqldb.DB, error) { return transaction, nil },
		nil,
	)
	if err != nil {
		t.Fatalf("HandleCrud: %v", err)
	}

	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("legacy HandleCrud did not broadcast before transaction completion")
	}

	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorCommitBroadcastsOnce(t *testing.T) {
	executor, mock, rootDB := beginTransactionalCrudTest(t, nil)
	tableName := t.Name()
	calls := make(chan sqldb.DB, 2)
	handler := func(_ http.Header, db sqldb.DB, _ *protodb.CrudReq, _ proto.Message, _ proto.Message) {
		calls <- db
	}
	GlobalCrudBroadcaster.RegisterBroadcast(tableName, handler)
	t.Cleanup(func() { GlobalCrudBroadcaster.UnregisterBroadcast(tableName, handler) })

	event := snapshotCrudBroadcastEvent(
		nil,
		executor.broadcastDB,
		&protodb.CrudReq{TableName: tableName, Code: protodb.CrudReqCode_INSERT},
		&protodb.CrudResp{},
		&protodb.CrudResp{RowsAffected: 1},
	)
	event.handlers = GlobalCrudBroadcaster.crudBroadcastHandlers(event.req)
	executor.pending = append(executor.pending, event)
	assertNoTransactionalBroadcast(t, calls)

	mock.ExpectCommit()
	if err := executor.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	select {
	case broadcastDB := <-calls:
		wrapped, ok := broadcastDB.(*sqldb.DBWithDialect)
		if !ok {
			t.Fatalf("broadcast db type = %T, want *sqldb.DBWithDialect", broadcastDB)
		}
		if wrapped.Executor != rootDB {
			t.Fatalf("broadcast executor = %T %p, want root db %p", wrapped.Executor, wrapped.Executor, rootDB)
		}
	case <-time.After(time.Second):
		t.Fatal("broadcast handler was not called after commit")
	}

	if err := executor.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("second Commit error = %v, want sql.ErrTxDone", err)
	}
	if err := executor.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Rollback after commit error = %v, want sql.ErrTxDone", err)
	}
	assertNoTransactionalBroadcast(t, calls)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorCommitErrorDiscardsBroadcasts(t *testing.T) {
	executor, mock, _ := beginTransactionalCrudTest(t, nil)
	tableName := t.Name()
	calls := make(chan struct{}, 1)
	handler := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		calls <- struct{}{}
	}
	GlobalCrudBroadcaster.RegisterBroadcast(tableName, handler)
	t.Cleanup(func() { GlobalCrudBroadcaster.UnregisterBroadcast(tableName, handler) })
	event := snapshotCrudBroadcastEvent(
		nil, executor.broadcastDB, &protodb.CrudReq{TableName: tableName}, nil, nil,
	)
	event.handlers = GlobalCrudBroadcaster.crudBroadcastHandlers(event.req)
	executor.pending = append(executor.pending, event)

	commitErr := errors.New("commit failed")
	mock.ExpectCommit().WillReturnError(commitErr)
	if err := executor.Commit(); err != commitErr {
		t.Fatalf("Commit error = %v, want original error %v", err, commitErr)
	}
	if len(executor.pending) != 0 {
		t.Fatalf("pending events = %d, want 0", len(executor.pending))
	}
	if err := executor.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Rollback after failed commit error = %v, want sql.ErrTxDone", err)
	}
	assertNoTransactionalBroadcast(t, calls)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorCommitPreservesCrudOrder(t *testing.T) {
	executor, mock, _ := beginTransactionalCrudTest(t, nil)
	tableName := t.Name()
	calls := make(chan string, 2)
	handler := func(_ http.Header, _ sqldb.DB, req *protodb.CrudReq, _ proto.Message, _ proto.Message) {
		calls <- string(req.MsgBytes)
	}
	GlobalCrudBroadcaster.RegisterBroadcast(tableName, handler)
	t.Cleanup(func() { GlobalCrudBroadcaster.UnregisterBroadcast(tableName, handler) })

	for _, value := range []string{"first", "second"} {
		event := snapshotCrudBroadcastEvent(
			nil,
			executor.broadcastDB,
			&protodb.CrudReq{TableName: tableName, Code: protodb.CrudReqCode_UPDATE, MsgBytes: []byte(value)},
			nil,
			nil,
		)
		event.handlers = GlobalCrudBroadcaster.crudBroadcastHandlers(event.req)
		executor.pending = append(executor.pending, event)
	}

	mock.ExpectCommit()
	if err := executor.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-calls:
			if got != want {
				t.Fatalf("broadcast order value = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q broadcast", want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorRollbackDiscardsBroadcasts(t *testing.T) {
	executor, mock, _ := beginTransactionalCrudTest(t, nil)
	calls := make(chan struct{}, 1)
	executor.pending = append(executor.pending, snapshotCrudBroadcastEvent(
		nil, executor.broadcastDB, &protodb.CrudReq{TableName: t.Name()}, nil, nil,
	))

	mock.ExpectRollback()
	if err := executor.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(executor.pending) != 0 {
		t.Fatalf("pending events = %d, want 0", len(executor.pending))
	}
	if err := executor.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Commit after rollback error = %v, want sql.ErrTxDone", err)
	}
	assertNoTransactionalBroadcast(t, calls)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorFailedCrudThenRollbackDiscardsEarlierSuccess(t *testing.T) {
	registerTransactionalCrudTestMessage()
	executor, mock, _ := beginTransactionalCrudTest(t, nil)
	calls := make(chan struct{}, 1)
	handler := func(http.Header, sqldb.DB, *protodb.CrudReq, proto.Message, proto.Message) {
		calls <- struct{}{}
	}
	GlobalCrudBroadcaster.RegisterBroadcast(transactionalCrudTestTable, handler)
	t.Cleanup(func() { GlobalCrudBroadcaster.UnregisterBroadcast(transactionalCrudTestTable, handler) })

	mock.ExpectExec("INSERT INTO CrudResp").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := executor.HandleCrud(context.Background(), nil, transactionalCrudInsertRequest(t, 1)); err != nil {
		t.Fatalf("first HandleCrud: %v", err)
	}

	failedRequest := transactionalCrudInsertRequest(t, 2)
	failedRequest.ResultType = protodb.CrudResultType(99)
	if _, err := executor.HandleCrud(context.Background(), nil, failedRequest); err == nil {
		t.Fatal("second HandleCrud returned nil error")
	}

	mock.ExpectRollback()
	if err := executor.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	assertNoTransactionalBroadcast(t, calls)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorHandleCrudDefersAndSnapshotsBroadcast(t *testing.T) {
	registerTransactionalCrudTestMessage()
	executor, mock, rootDB := beginTransactionalCrudTest(t, nil)
	calls := make(chan error, 1)
	handler := func(meta http.Header, db sqldb.DB, req *protodb.CrudReq, reqMsg proto.Message, respMsg proto.Message) {
		if meta.Get("X-Test") != "before" {
			calls <- errors.New("broadcast header was not snapshotted")
			return
		}
		if req.TableName != transactionalCrudTestTable || string(req.MsgBytes) == "changed" {
			calls <- errors.New("broadcast request was not snapshotted")
			return
		}
		wrapped, ok := db.(*sqldb.DBWithDialect)
		if !ok || wrapped.Executor != rootDB {
			calls <- errors.New("broadcast did not receive the root database executor")
			return
		}
		requestMessage, ok := reqMsg.(*protodb.CrudResp)
		if !ok || requestMessage.RowsAffected != 7 {
			calls <- errors.New("broadcast request message was not snapshotted")
			return
		}
		responseMessage, ok := respMsg.(*protodb.CrudResp)
		if !ok || responseMessage.RowsAffected != 1 {
			calls <- errors.New("broadcast response message was not snapshotted")
			return
		}
		calls <- nil
	}
	GlobalCrudBroadcaster.RegisterBroadcast(transactionalCrudTestTable, handler)
	t.Cleanup(func() { GlobalCrudBroadcaster.UnregisterBroadcast(transactionalCrudTestTable, handler) })

	msgBytes, err := proto.Marshal(&protodb.CrudResp{RowsAffected: 7})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	meta := http.Header{"X-Test": []string{"before"}}
	req := &protodb.CrudReq{
		Code:       protodb.CrudReqCode_INSERT,
		ResultType: protodb.CrudResultType_DMLResult,
		TableName:  transactionalCrudTestTable,
		MsgBytes:   msgBytes,
	}
	mock.ExpectExec("INSERT INTO CrudResp").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := executor.HandleCrud(context.Background(), meta, req); err != nil {
		t.Fatalf("HandleCrud: %v", err)
	}
	if len(executor.pending) != 1 {
		t.Fatalf("pending events = %d, want 1", len(executor.pending))
	}
	assertNoTransactionalBroadcast(t, calls)
	GlobalCrudBroadcaster.UnregisterBroadcast(transactionalCrudTestTable, handler)

	meta.Set("X-Test", "after")
	req.TableName = "changed"
	req.MsgBytes = []byte("changed")
	mock.ExpectCommit()
	if err := executor.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	select {
	case err := <-calls:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("broadcast handler was not called")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorPermissionFailureDoesNotQueueBroadcast(t *testing.T) {
	registerTransactionalCrudTestMessage()
	permissionErr := errors.New("permission denied")
	executor, mock, _ := beginTransactionalCrudTest(t, func(http.Header, string, protodb.CrudReqCode, sqldb.DB, proto.Message) error {
		return permissionErr
	})
	msgBytes, err := proto.Marshal(&protodb.CrudResp{})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	_, err = executor.HandleCrud(context.Background(), nil, &protodb.CrudReq{
		Code:       protodb.CrudReqCode_INSERT,
		ResultType: protodb.CrudResultType_DMLResult,
		TableName:  transactionalCrudTestTable,
		MsgBytes:   msgBytes,
	})
	if err == nil {
		t.Fatal("HandleCrud returned nil error")
	}
	if len(executor.pending) != 0 {
		t.Fatalf("pending events = %d, want 0", len(executor.pending))
	}

	mock.ExpectRollback()
	if err := executor.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorDBUsesTransactionAndDoesNotBroadcast(t *testing.T) {
	executor, mock, _ := beginTransactionalCrudTest(t, nil)
	if executor.DB() == nil {
		t.Fatal("DB returned nil for open executor")
	}
	mock.ExpectExec("UPDATE audit SET value").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := executor.DB().ExecContext(context.Background(), "UPDATE audit SET value = ?", "changed"); err != nil {
		t.Fatalf("DB().ExecContext: %v", err)
	}
	if len(executor.pending) != 0 {
		t.Fatalf("pending events = %d, want 0", len(executor.pending))
	}
	mock.ExpectRollback()
	if err := executor.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionalCrudExecutorNilAndClosedReceivers(t *testing.T) {
	var executor *TransactionalCrudExecutor
	if executor.DB() != nil {
		t.Fatal("nil executor DB returned non-nil")
	}
	if _, err := executor.HandleCrud(context.Background(), nil, nil); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("nil executor HandleCrud error = %v, want sql.ErrTxDone", err)
	}
	if err := executor.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("nil executor Commit error = %v, want sql.ErrTxDone", err)
	}
	if err := executor.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("nil executor Rollback error = %v, want sql.ErrTxDone", err)
	}
	zero := &TransactionalCrudExecutor{}
	if zero.DB() != nil {
		t.Fatal("zero executor DB returned non-nil")
	}
	if _, err := zero.HandleCrud(context.Background(), nil, nil); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("zero executor HandleCrud error = %v, want sql.ErrTxDone", err)
	}
	if err := zero.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("zero executor Commit error = %v, want sql.ErrTxDone", err)
	}
	if err := zero.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("zero executor Rollback error = %v, want sql.ErrTxDone", err)
	}

	executor, mock, _ := beginTransactionalCrudTest(t, nil)
	if _, err := executor.HandleCrud(context.Background(), nil, nil); !errors.Is(err, errNilTransactionalCrudRequest) {
		t.Fatalf("nil request error = %v, want %v", err, errNilTransactionalCrudRequest)
	}
	mock.ExpectRollback()
	if err := executor.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if executor.DB() != nil {
		t.Fatal("DB after rollback returned non-nil")
	}
	if _, err := executor.HandleCrud(context.Background(), nil, nil); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("HandleCrud after rollback error = %v, want sql.ErrTxDone", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
