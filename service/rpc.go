package service

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/crud"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/querystore"
	"google.golang.org/protobuf/proto"
)

type TrpcManager struct {
	protodb.UnimplementedProtoDbSrvHandler
	FnGetDb crud.TfnProtodbGetDb

	// proto.message name => fn
	// must set for every protodb message, if no fn for a message, set to nil
	fnCrudPermissionMap map[string]crud.TfnProtodbCrudPermission

	// table name => fn
	// must set for every table, if no fn for a table, set to nil
	fnTableQueryPermissionMap map[string]crud.TfnTableQueryPermission
}

// NewTrpcManager create new manager for rpc
func NewTrpcManager(fnGetCrudDb crud.TfnProtodbGetDb, fnCrudPermission map[string]crud.TfnProtodbCrudPermission,
	fnTableQueryPermission map[string]crud.TfnTableQueryPermission) *TrpcManager {
	// set default value
	if fnGetCrudDb == nil {
		fnGetCrudDb = crud.FnProtodbGetDbEmpty
	}
	if fnCrudPermission == nil {
		fnCrudPermission = make(map[string]crud.TfnProtodbCrudPermission)
	}
	if fnTableQueryPermission == nil {
		fnTableQueryPermission = make(map[string]crud.TfnTableQueryPermission)
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

	fnCrudPermission := this.fnCrudPermissionMap[CrudMsg.TableName]

	if fnCrudPermission == nil && CrudMsg.Code != protodb.CrudReqCode_SELECTONE {
		return nil, fmt.Errorf("no crudpermission function for table %s", CrudMsg.TableName)
	}

	respCrud, err := crud.Crud(ctx, meta, CrudMsg, this.FnGetDb, fnCrudPermission)
	if err != nil {
		return nil, err
	}

	resp = &connect.Response[protodb.CrudResp]{
		Msg: respCrud,
	}

	return resp, nil

}

func (this *TrpcManager) TableQuery(ctx context.Context, req *connect.Request[protodb.TableQueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	sendErr := func(err error) error {
		resp := &protodb.QueryResp{
			ResponseNo:  0,
			ErrInfo:     err.Error(),
			MsgBytes:    nil,
			MsgFormat:   0,
			ResponseEnd: true,
		}
		return ss.Send(resp)
	}

	meta := req.Header()
	TableQueryReq := req.Msg

	permissionFn, ok := this.fnTableQueryPermissionMap[TableQueryReq.TableName]
	if !ok {
		return sendErr(fmt.Errorf("no permission check function for table %s", TableQueryReq.TableName))
	}

	return crud.TableQuery(ctx, meta, TableQueryReq, this.FnGetDb, permissionFn, ss.Send)
}

func (this *TrpcManager) Query(ctx context.Context, req *connect.Request[protodb.QueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {

	sendErr := func(err error) error {
		resp := &protodb.QueryResp{
			ResponseNo:  0,
			ErrInfo:     err.Error(),
			MsgBytes:    nil,
			MsgFormat:   0,
			ResponseEnd: true,
		}
		return ss.Send(resp)
	}

	db, err := this.FnGetDb(req.Header(), "", "", false)
	if err != nil {
		return sendErr(err)
	}

	fn, ok := querystore.GetQuery(req.Msg.QueryName)
	if !ok {
		return sendErr(fmt.Errorf("err: can not get query fn for %s", req.Msg.QueryName))
	}

	sqlStr, sqlVals, fnGetResultMsg, err := fn(req.Header(), db, req.Msg)
	if err != nil {
		return sendErr(fmt.Errorf("generate query sql for %s err: %w", req.Msg.QueryName, err))
	}

	var resultMsg proto.Message

	resultMsg = fnGetResultMsg(false)

	// Determine which fields to scan
	resultColumns := req.Msg.ResultColumnNames
	useAllFields := len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*")
	fieldNames := resultColumns
	if useAllFields {
		fieldNames = nil
	}

	msgDesc := resultMsg.ProtoReflect().Descriptor()
	msgFieldsMap := pdbutil.BuildMsgFieldsMap(fieldNames, msgDesc.Fields(), true)

	rows, err := db.Query(sqlStr, sqlVals...)
	if err != nil {
		return sendErr(fmt.Errorf("query %s err: %w", req.Msg.QueryName, err))
	}

	defer rows.Close()

	var respNo int64 = 0
	batchSize := req.Msg.PreferBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	if batchSize > 10000 {
		batchSize = 10000
	}
	respBatchSize := batchSize
	respMsgByteSize := 0
	maxMsgByteSize := 1024 * 1024
	resp := &protodb.QueryResp{
		ResponseNo:  respNo,
		MsgFormat:   req.Msg.MsgFormat,
		MsgBytes:    nil,
		ResponseEnd: false,
	}

	for rows.Next() {
		resultMsg = fnGetResultMsg(true)

		// Scan row data
		err = crud.DbScan2ProtoMsg(rows, resultMsg,
			fieldNames,
			msgFieldsMap,
		)
		if err != nil {
			return sendErr(fmt.Errorf("scan row data err: %w", err))
		}

		resultMsgBytes, err := crud.MsgMarshal(resultMsg, req.Msg.MsgFormat)
		if err != nil {
			return sendErr(fmt.Errorf("marshal msg err: %w", err))
		}
		resp.MsgBytes = append(resp.MsgBytes, resultMsgBytes)
		respMsgByteSize += len(resultMsgBytes)
		respBatchSize++
		if respMsgByteSize >= maxMsgByteSize || respBatchSize >= batchSize {
			resp.ResponseNo = respNo
			resp.ResponseEnd = false
			err = ss.Send(resp)
			if err != nil {
				return sendErr(fmt.Errorf("send msg fail, %w", err))
			}
			respNo++
			respBatchSize = 0
			respMsgByteSize = 0
			resp = &protodb.QueryResp{
				ResponseNo:  respNo,
				MsgFormat:   req.Msg.MsgFormat,
				MsgBytes:    nil,
				ResponseEnd: false,
			}
		}
	}

	err = rows.Err()
	if err != nil {
		return sendErr(fmt.Errorf("query %s err: %w", req.Msg.QueryName, err))
	}

	resp.ResponseEnd = true
	err = ss.Send(resp)
	if err != nil {
		return sendErr(fmt.Errorf("send msg fail, %w", err))
	}
	return nil

}
