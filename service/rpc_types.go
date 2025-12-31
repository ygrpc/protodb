package service

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

// TfnProtodbGetDb get db for db operation (legacy, returns *sql.DB)
// Deprecated: Use TfnProtodbGetDBExecutor for transaction support
// meta http header
// schemaName schema name
// tableName table name
// writable if db is writable
type TfnProtodbGetDb func(meta http.Header, schemaName string, tableName string, writable bool) (db *sql.DB, err error)

// TfnProtodbGetDBExecutor get DBExecutor for db operation (supports transactions)
// meta http header
// schemaName schema name
// tableName table name
// writable if db is writable
// Returns sqldb.DBExecutor which can be *sql.DB, *sql.Tx, or *sqldb.DBWithDialect
type TfnProtodbGetDBExecutor func(meta http.Header, schemaName string, tableName string, writable bool) (db sqldb.DBExecutor, err error)

// FnProtodbGetDbEmpty return nil (legacy)
func FnProtodbGetDbEmpty(meta http.Header, schemaName string, tableName string, writable bool) (db *sql.DB, err error) {
	return nil, errors.New("FnProtodbGetDbEmpty")
}

// FnProtodbGetDBExecutorEmpty return nil
func FnProtodbGetDBExecutorEmpty(meta http.Header, schemaName string, tableName string, writable bool) (db sqldb.DBExecutor, err error) {
	return nil, errors.New("FnProtodbGetDBExecutorEmpty")
}

// TfnProtodbCrudPermission permission check function for CRUD operations (legacy, uses *sql.DB)
// Deprecated: Use TfnProtodbCrudPermissionExecutor for transaction support
type TfnProtodbCrudPermission func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) (err error)

// TfnProtodbCrudPermissionExecutor permission check function for CRUD operations (supports transactions)
type TfnProtodbCrudPermissionExecutor func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DBExecutor, dbmsg proto.Message) (err error)

// FnProtodbCrudPermissionEmpty allow all crud operation (legacy)
func FnProtodbCrudPermissionEmpty(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) (err error) {
	return nil
}

// FnProtodbCrudPermissionExecutorEmpty allow all crud operation
func FnProtodbCrudPermissionExecutorEmpty(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DBExecutor, dbmsg proto.Message) (err error) {
	return nil
}

// TfnTableQueryPermission permission check function for table query (legacy, uses *sql.DB)
// Deprecated: Use TfnTableQueryPermissionExecutor for transaction support
type TfnTableQueryPermission func(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (wherSqlStr string, whereSqlVals []any, err error)

// TfnTableQueryPermissionExecutor permission check function for table query (supports transactions)
type TfnTableQueryPermissionExecutor func(meta http.Header, schemaName string, tableName string, db sqldb.DBExecutor, dbmsg proto.Message) (wherSqlStr string, whereSqlVals []any, err error)

// FnTableQueryPermissionEmpty empty where, allow query all rows (legacy)
func FnTableQueryPermissionEmpty(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (wherStr string, whereSqlVals []any, err error) {
	return "", nil, nil
}

// FnTableQueryPermissionExecutorEmpty empty where, allow query all rows
func FnTableQueryPermissionExecutorEmpty(meta http.Header, schemaName string, tableName string, db sqldb.DBExecutor, dbmsg proto.Message) (wherStr string, whereSqlVals []any, err error) {
	return "", nil, nil
}

type TfnSendQueryResp func(resp *protodb.QueryResp) error
