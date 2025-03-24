package crud

import (
	"connectrpc.com/connect"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ygrpc/protodb/msgstore"
	"net/http"

	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
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

func Crud(ctx context.Context, meta http.Header, req *protodb.CrudReq, fnGetDb TfnProtodbGetDb, fnCrudPermission TfnProtodbCrudPermission, fnTableQueryPermission TfnTableQueryPermission) (resp *protodb.CrudResp, err error) {

	db, err := fnGetDb(meta, req.SchemeName, req.TableName, true)
	if err != nil {
		return nil, err
	}

	dbmsg, ok := msgstore.GetMsg(req.TableName, true)
	if !ok {
		return nil, fmt.Errorf("can not get proto msg %s err", req.TableName)
	}

	// unmarshal
	err = proto.Unmarshal(req.MsgBytes, dbmsg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal msg %s err: %w", req.TableName, err)
	}

	if fnCrudPermission != nil {
		err = fnCrudPermission(meta, req.SchemeName, req.Code, db, dbmsg)
		if err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
	}

	switch req.Code {
	case protodb.CrudReqCode_INSERT:
		switch req.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := DbInsert(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("insert msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := DbInsertReturn(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("insert msg %s err: %w", req.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", req.TableName, err)
			}
			resp = &protodb.CrudResp{
				RowsAffected: 1,
				NewMsgBytes:  NewMsgBytes,
			}

			return resp, nil
		}
	case protodb.CrudReqCode_UPDATE:
		switch req.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := DbUpdate(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult

			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := DbUpdateReturnNew(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", req.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", req.TableName, err)
			}
			resp = &protodb.CrudResp{
				RowsAffected: 1,
				NewMsgBytes:  NewMsgBytes,
			}
			return resp, nil
		case protodb.CrudResultType_OldMsgAndNewMsg:
			oldMsg, newMsg, err := DbUpdateReturnOldAndNew(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", req.TableName, err)
			}
			OldMsgBytes, err := proto.Marshal(oldMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal old msg %s err: %w", req.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", req.TableName, err)
			}
			resp = &protodb.CrudResp{
				RowsAffected: 1,
				OldMsgBytes:  OldMsgBytes,
				NewMsgBytes:  NewMsgBytes,
			}
			return resp, nil
		}
	case protodb.CrudReqCode_DELETE:
		switch req.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := DbDelete(db, dbmsg, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("delete msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := DbDeleteReturn(db, dbmsg, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("delete msg %s err: %w", req.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", req.TableName, err)
			}
			resp = &protodb.CrudResp{
				RowsAffected: 1,
				NewMsgBytes:  NewMsgBytes,
			}
			return resp, nil
		}
	case protodb.CrudReqCode_SELECTONE:
		newMsg, err := DbSelectOne(db, dbmsg, req.SelectOneKeyFields, req.SelectResultFields, req.SchemeName)
		if err != nil {
			return nil, fmt.Errorf("selectone msg %s err: %w", req.TableName, err)
		}
		NewMsgBytes, err := proto.Marshal(newMsg)
		if err != nil {
			return nil, fmt.Errorf("marshal new msg %s err: %w", req.TableName, err)
		}
		resp = &protodb.CrudResp{
			RowsAffected: 1,
			NewMsgBytes:  NewMsgBytes,
		}
		return resp, nil
	}

	return nil, fmt.Errorf("Unknown crud code: %s", req.Code.String())
}
