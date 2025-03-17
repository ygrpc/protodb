package service

import (
	"connectrpc.com/connect"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/crud"
	"github.com/ygrpc/protodb/msgstore"
	"google.golang.org/protobuf/proto"
	"net/http"
)

type TfnProtodbGetDb func(meta http.Header, schemaName string) (db *sql.DB, err error)

type TfnProtodbCheckPermission func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) (err error)

// FnProtodbCheckPermissionEmpty allow all crud operation
func FnProtodbCheckPermissionEmpty(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db *sql.DB, dbmsg proto.Message) (err error) {
	return nil
}

type TfnTableQueryPermission func(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (wherStr string, err error)

// FnTableQueryPermissionEmpty empty where, allow query all rows
func FnTableQueryPermissionEmpty(meta http.Header, schemaName string, tableName string, db *sql.DB, dbmsg proto.Message) (wherStr string, err error) {
	return "", nil
}

type TrpcManager struct {
	protodb.UnimplementedProtoDbSrvHandler
	FnGetCrudDb TfnGetCrudDb
	// proto.message name => fn
	FnCheckCrudPermission map[string]TfnProtodbCheckPermission

	// table name => fn
	FnTableQueryPermission map[string]TfnTableQueryPermission
}

// NewTrpcManager create new manager for rpc
func NewTrpcManager(fnGetCrudDb TfnProtodbGetDb, fnCheckCrudPermission map[string]TfnProtodbCheckPermission) *TrpcManager {
	return &TrpcManager{
		FnGetDb:               fnGetCrudDb,
		FnCheckCrudPermission: fnCheckCrudPermission,
	}
}

func (this *TrpcManager) Crud(ctx context.Context, req *connect.Request[protodb.CrudReq]) (resp *connect.Response[protodb.CrudResp], err error) {
	meta := req.Header()
	CrudMsg := req.Msg

	if this.FnGetDb == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("FnGetDb is nil"))
	}
	db, err := this.FnGetDb(meta, CrudMsg.SchemeName)
	if err != nil {
		return nil, err
	}

	dbmsg, ok := msgstore.GetMsg(CrudMsg.TableName, true)
	if !ok {
		return nil, fmt.Errorf("can not get proto msg %s err", CrudMsg.TableName)
	}

	// unmarshal
	err = proto.Unmarshal(CrudMsg.MsgBytes, dbmsg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal msg %s err: %w", CrudMsg.TableName, err)
	}

	if this.FnCheckCrudPermission != nil {
		if fncheck, ok := this.FnCheckCrudPermission[CrudMsg.TableName]; ok {
			err = fncheck(meta, CrudMsg.SchemeName, CrudMsg.Code, db, dbmsg)
			if err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, err)
			}
		}
	}

	switch CrudMsg.Code {
	case protodb.CrudReqCode_INSERT:
		switch CrudMsg.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := crud.DbInsert(db, dbmsg, CrudMsg.MsgLastFieldNo, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("insert msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: dmlResult,
			}
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbInsertReturn(db, dbmsg, CrudMsg.MsgLastFieldNo, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("insert msg %s err: %w", CrudMsg.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: &protodb.CrudResp{
					RowsAffected: 1,
					NewMsgBytes:  NewMsgBytes,
				},
			}
			return resp, nil
		}
	case protodb.CrudReqCode_UPDATE:
		switch CrudMsg.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := crud.DbUpdate(db, dbmsg, CrudMsg.MsgLastFieldNo, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: dmlResult,
			}
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbUpdateReturnNew(db, dbmsg, CrudMsg.MsgLastFieldNo, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", CrudMsg.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: &protodb.CrudResp{
					RowsAffected: 1,
					NewMsgBytes:  NewMsgBytes,
				},
			}
			return resp, nil
		case protodb.CrudResultType_OldMsgAndNewMsg:
			oldMsg, newMsg, err := crud.DbUpdateReturnOldAndNew(db, dbmsg, CrudMsg.MsgLastFieldNo, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", CrudMsg.TableName, err)
			}
			OldMsgBytes, err := proto.Marshal(oldMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal old msg %s err: %w", CrudMsg.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: &protodb.CrudResp{
					RowsAffected: 1,
					OldMsgBytes:  OldMsgBytes,
					NewMsgBytes:  NewMsgBytes,
				},
			}
			return resp, nil
		}
	case protodb.CrudReqCode_DELETE:
		switch CrudMsg.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := crud.DbDelete(db, dbmsg, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("delete msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: dmlResult,
			}
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbDeleteReturn(db, dbmsg, CrudMsg.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("delete msg %s err: %w", CrudMsg.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", CrudMsg.TableName, err)
			}
			resp = &connect.Response[protodb.CrudResp]{
				Msg: &protodb.CrudResp{
					RowsAffected: 1,
					NewMsgBytes:  NewMsgBytes,
				},
			}
			return resp, nil
		}
	case protodb.CrudReqCode_SELECTONE:
		newMsg, err := crud.DbSelectOne(db, dbmsg, CrudMsg.SelectOneKeyFields, CrudMsg.SelectResultFields, CrudMsg.SchemeName)
		if err != nil {
			return nil, fmt.Errorf("selectone msg %s err: %w", CrudMsg.TableName, err)
		}
		NewMsgBytes, err := proto.Marshal(newMsg)
		if err != nil {
			return nil, fmt.Errorf("marshal new msg %s err: %w", CrudMsg.TableName, err)
		}
		resp = &connect.Response[protodb.CrudResp]{
			Msg: &protodb.CrudResp{
				RowsAffected: 1,
				NewMsgBytes:  NewMsgBytes,
			},
		}
		return resp, nil
	}

	return nil, fmt.Errorf("Unknown crud code: %s", CrudMsg.Code.String())
}

func (this *TrpcManager) TableQuery(ctx context.Context, req *connect.Request[protodb.TableQueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("protodb.ProtoDbSrv.TableQuery is not implemented"))
}

func (this *TrpcManager) Query(ctx context.Context, req *connect.Request[protodb.QueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("protodb.ProtoDbSrv.Query is not implemented"))
}
