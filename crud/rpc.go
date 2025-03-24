package crud

import (
	"database/sql"
	"errors"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
	"net/http"
)

// TfnProtodbGetDb get db for db operation
// meta http header
// schemaName schema name
// tableName table name
// writable if db is writable
type TfnProtodbGetDb func(meta http.Header, schemaName string, tableName string, writable bool) (db *sql.DB, err error)

// FnProtodbGetDbEmpty return nil
func FnProtodbGetDbEmpty(meta http.Header, schemaName string, tableName string, writable bool) (db *sql.DB, err error) {
	return nil, errors.New("FnProtodbGetDbEmpty")
}

type TfnProtodbCrudPermission func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) (err error)

// FnProtodbCrudPermissionEmpty allow all crud operation
func FnProtodbCrudPermissionEmpty(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) (err error) {
	return nil
}

type TfnTableQueryPermission func(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (wherStr string, err error)

// FnTableQueryPermissionEmpty empty where, allow query all rows
func FnTableQueryPermissionEmpty(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (wherStr string, err error) {
	return "", nil
}
