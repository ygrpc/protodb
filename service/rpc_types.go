package service

import (
	"errors"
	"net/http"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
)

// TfnProtodbGetDb get DB for db operation (supports transactions)
// meta http header
// schemaName schema name
// tableName table name
// writable if db is writable
// Returns sqldb.DB which can be *sql.DB, *sql.Tx, or *sqldb.DBWithDialect
type TfnProtodbGetDb func(meta http.Header, schemaName string, tableName string, writable bool) (db sqldb.DB, err error)

// FnProtodbGetDbEmpty return nil
func FnProtodbGetDbEmpty(meta http.Header, schemaName string, tableName string, writable bool) (db sqldb.DB, err error) {
	return nil, errors.New("FnProtodbGetDbEmpty")
}

// TfnProtodbCrudPermission permission check function for CRUD operations (supports transactions)
type TfnProtodbCrudPermission func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DB, dbmsg proto.Message) (err error)

// FnProtodbCrudPermissionEmpty allow all crud operation
func FnProtodbCrudPermissionEmpty(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DB, dbmsg proto.Message) (err error) {
	return nil
}

// TfnTableQueryPermission permission check function for table query (supports transactions)
type TfnTableQueryPermission func(meta http.Header, schemaName string, tableName string, db sqldb.DB, dbmsg proto.Message) (wherSqlStr string, whereSqlVals []any, err error)

// FnTableQueryPermissionEmpty empty where, allow query all rows
func FnTableQueryPermissionEmpty(meta http.Header, schemaName string, tableName string, db sqldb.DB, dbmsg proto.Message) (wherStr string, whereSqlVals []any, err error) {
	return "", nil, nil
}

type TfnSendQueryResp func(resp *protodb.QueryResp) error
