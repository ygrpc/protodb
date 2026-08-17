package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
)

var (
	errNilTransactionalCrudDB      = errors.New("service: transactional CRUD database is nil")
	errNilTransactionalCrudRequest = errors.New("service: transactional CRUD request is nil")
)

type transactionalCrudState uint8

const (
	transactionalCrudOpen transactionalCrudState = iota
	transactionalCrudCommitted
	transactionalCrudRolledBack
	transactionalCrudCommitFailed
)

// TransactionalCrudExecutor executes ProtoDb CRUD operations in one database
// transaction and delays their broadcasts until Commit succeeds.
//
// A TransactionalCrudExecutor is single-use. It must not be used concurrently
// by multiple goroutines, and HandleCrud must not run concurrently with Commit
// or Rollback.
type TransactionalCrudExecutor struct {
	tx          *sql.Tx
	transaction *sqldb.DBWithDialect
	broadcastDB *sqldb.DBWithDialect
	permission  TfnProtodbCrudPermission

	state   transactionalCrudState
	pending []crudBroadcastEvent
}

// BeginTransactionalCrud starts one transaction-backed CRUD executor.
// Permission, when non-nil, is applied to every HandleCrud call made through
// the returned executor.
func BeginTransactionalCrud(
	ctx context.Context,
	db *sql.DB,
	options *sql.TxOptions,
	permission TfnProtodbCrudPermission,
) (*TransactionalCrudExecutor, error) {
	if db == nil {
		return nil, errNilTransactionalCrudDB
	}

	tx, err := db.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}

	return &TransactionalCrudExecutor{
		tx:          tx,
		transaction: sqldb.NewTxWithDialect(tx, db),
		broadcastDB: sqldb.NewDBWithDialect(db),
		permission:  permission,
		state:       transactionalCrudOpen,
	}, nil
}

// DB returns the database executor bound to an open transaction. SQL executed
// directly through DB participates in the transaction but is not broadcast.
// DB returns nil after the transaction has completed.
func (e *TransactionalCrudExecutor) DB() sqldb.DB {
	if e == nil || e.state != transactionalCrudOpen || e.transaction == nil {
		return nil
	}
	return e.transaction
}

// HandleCrud executes one CRUD request in the transaction. Successful write
// operations are snapshotted for broadcast after a successful Commit. Read
// operations and failed CRUD operations are never queued for broadcast.
func (e *TransactionalCrudExecutor) HandleCrud(
	ctx context.Context,
	meta http.Header,
	req *protodb.CrudReq,
) (*protodb.CrudResp, error) {
	if e == nil || e.state != transactionalCrudOpen || e.transaction == nil || e.broadcastDB == nil {
		return nil, sql.ErrTxDone
	}
	if req == nil {
		return nil, errNilTransactionalCrudRequest
	}

	resp, requestMessage, err := executeCrud(ctx, meta, req, e.transaction, e.permission)
	if err != nil {
		return nil, err
	}

	if crudRequestIsBroadcastWrite(req) {
		event := snapshotCrudBroadcastEvent(
			meta,
			e.broadcastDB,
			req,
			requestMessage,
			resp,
		)
		// Keep the execution-time recipients; Commit only controls delivery timing.
		event.handlers = GlobalCrudBroadcaster.crudBroadcastHandlers(event.req)
		e.pending = append(e.pending, event)
	}

	return resp, nil
}

// Commit commits the underlying transaction. Pending CRUD events are
// broadcast asynchronously, in execution order, only when the underlying
// Commit returns nil.
func (e *TransactionalCrudExecutor) Commit() error {
	if e == nil || e.state != transactionalCrudOpen || e.tx == nil {
		return sql.ErrTxDone
	}

	if err := e.tx.Commit(); err != nil {
		e.pending = nil
		e.state = transactionalCrudCommitFailed
		return err
	}

	events := e.pending
	e.pending = nil
	e.state = transactionalCrudCommitted
	GlobalCrudBroadcaster.broadcastEventsAsync(events)
	return nil
}

// Rollback rolls back the underlying transaction and discards every pending
// CRUD broadcast. An executor cannot be reused after Rollback returns.
func (e *TransactionalCrudExecutor) Rollback() error {
	if e == nil || e.state != transactionalCrudOpen || e.tx == nil {
		return sql.ErrTxDone
	}

	e.pending = nil
	e.state = transactionalCrudRolledBack
	return e.tx.Rollback()
}

func crudRequestIsBroadcastWrite(req *protodb.CrudReq) bool {
	if req == nil {
		return false
	}

	switch req.Code {
	case protodb.CrudReqCode_INSERT,
		protodb.CrudReqCode_UPDATE,
		protodb.CrudReqCode_PARTIALUPDATE,
		protodb.CrudReqCode_DELETE:
		return true
	default:
		return false
	}
}
