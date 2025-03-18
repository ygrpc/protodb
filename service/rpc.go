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
	"google.golang.org/protobuf/encoding/protojson"
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

type TrpcManager struct {
	protodb.UnimplementedProtoDbSrvHandler
	FnGetDb TfnProtodbGetDb

	// proto.message name => fn
	// must set for every protodb message, if no fn for a message, set to nil
	fnCrudPermissionMap map[string]TfnProtodbCrudPermission

	// table name => fn
	// must set for every table, if no fn for a table, set to nil
	fnTableQueryPermissionMap map[string]TfnTableQueryPermission
}

// NewTrpcManager create new manager for rpc
func NewTrpcManager(fnGetCrudDb TfnProtodbGetDb, fnCrudPermission map[string]TfnProtodbCrudPermission,
	fnTableQueryPermission map[string]TfnTableQueryPermission) *TrpcManager {
	// set default value
	if fnGetCrudDb == nil {
		fnGetCrudDb = FnProtodbGetDbEmpty
	}
	if fnCrudPermission == nil {
		fnCrudPermission = make(map[string]TfnProtodbCrudPermission)
	}
	if fnTableQueryPermission == nil {
		fnTableQueryPermission = make(map[string]TfnTableQueryPermission)
	}
	return &TrpcManager{
		FnGetDb:                   fnGetCrudDb,
		fnCrudPermissionMap:       fnCrudPermission,
		fnTableQueryPermissionMap: fnTableQueryPermission,
	}
}

func (this *TrpcManager) Crud(ctx context.Context, req *connect.Request[protodb.CrudReq]) (resp *connect.Response[protodb.CrudResp], err error) {
	meta := req.Header()
	CrudMsg := req.Msg

	db, err := this.FnGetDb(meta, CrudMsg.SchemeName, CrudMsg.TableName, true)
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

	if fnCrudPermission, ok := this.fnCrudPermissionMap[CrudMsg.TableName]; ok {
		if fnCrudPermission != nil {
			err = fnCrudPermission(meta, CrudMsg.SchemeName, CrudMsg.Code, db, dbmsg)
			if err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, err)
			}
		}
	} else {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("no crudpermission function for table %s", CrudMsg.TableName))
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
	meta := req.Header()
	TableQueryReq := req.Msg

	db, err := this.FnGetDb(meta, TableQueryReq.SchemeName, TableQueryReq.TableName, false)
	if err != nil {
		return err
	}

	dbmsg, ok := msgstore.GetMsg(TableQueryReq.TableName, false)
	if !ok {
		return fmt.Errorf("can not get protodb msg %s err", TableQueryReq.TableName)
	}

	permissionSqlStr := ""

	permissionFn, ok := this.fnTableQueryPermissionMap[TableQueryReq.TableName]
	if ok {
		if permissionFn != nil {
			permissionSqlStr, err = permissionFn(meta, TableQueryReq.SchemeName, TableQueryReq.TableName, db, dbmsg)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	} else {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("no permission check function for table %s", TableQueryReq.TableName))
	}

	batchSize := TableQueryReq.PreferBatchSize
	if batchSize <= 0 || batchSize > 10000 {
		batchSize = 1
	}
	resultCh := make(chan crud.TqueryItem, batchSize)

	err = crud.DbTableQuery(db, dbmsg, TableQueryReq.Where, TableQueryReq.ResultColumnNames, TableQueryReq.SchemeName, TableQueryReq.TableName, permissionSqlStr, resultCh)
	if err != nil {
		return fmt.Errorf("table query msg %s err: %w", TableQueryReq.TableName, err)
	}

	var responseNo int64 = 0
	msgBytes := make([][]byte, 0)

	var tmpQueryResp *protodb.QueryResp

	for item := range resultCh {
		if item.Err != nil {
			return connect.NewError(connect.CodeInternal, errors.New(*item.Err))
		}

		var tmpMarshalByte []byte
		if TableQueryReq.MsgFormat == 0 {
			tmpMarshalByte, err = proto.Marshal(item.Msg)
			if err != nil {
				return fmt.Errorf("marshal bytes error: %w", err)
			}
		} else {
			tmpMarshalByte, err = protojson.Marshal(item.Msg)
			if err != nil {
				return fmt.Errorf("marshal json error: %w", err)
			}
		}

		msgBytes = append(msgBytes, tmpMarshalByte)
		if len(msgBytes) >= int(batchSize) || item.IsEnd {
			tmpQueryResp = &protodb.QueryResp{
				ResponseNo: responseNo,
				MsgBytes:   msgBytes,
				MsgFormat:  TableQueryReq.MsgFormat,
			}
			if item.IsEnd {
				tmpQueryResp.ResponseEnd = true
			}
			err = ss.Send(tmpQueryResp)
			if err != nil {
				return fmt.Errorf("send msg fail, %w", err)
			}
			msgBytes = make([][]byte, batchSize)
			responseNo++
		}
	}

	return nil

}

func (this *TrpcManager) Query(ctx context.Context, req *connect.Request[protodb.QueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("protodb.ProtoDbSrv.Query is not implemented"))
}
