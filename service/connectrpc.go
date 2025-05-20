package service

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/crud"
)

type TconnectrpcProtoDbSrvHandlerImpl struct {
	protodb.UnimplementedProtoDbSrvHandler
	FnGetDb crud.TfnProtodbGetDb

	// proto.message name => fn
	// must set for every protodb message, if no fn for a message, set to nil
	fnCrudPermissionMap map[string]crud.TfnProtodbCrudPermission

	// table name => fn
	// must set for every table, if no fn for a table, set to nil
	fnTableQueryPermissionMap map[string]crud.TfnTableQueryPermission
}

// NewTconnectrpcProtoDbSrvHandlerImpl create new ProtoDbSrvHandler impl in connectrpc
func NewTconnectrpcProtoDbSrvHandlerImpl(fnGetCrudDb crud.TfnProtodbGetDb, fnCrudPermission map[string]crud.TfnProtodbCrudPermission,
	fnTableQueryPermission map[string]crud.TfnTableQueryPermission) *TconnectrpcProtoDbSrvHandlerImpl {
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
	return &TconnectrpcProtoDbSrvHandlerImpl{
		FnGetDb:                   fnGetCrudDb,
		fnCrudPermissionMap:       fnCrudPermission,
		fnTableQueryPermissionMap: fnTableQueryPermission,
	}
}

func (this *TconnectrpcProtoDbSrvHandlerImpl) Crud(ctx context.Context, req *connect.Request[protodb.CrudReq]) (resp *connect.Response[protodb.CrudResp], err error) {
	meta := req.Header()
	CrudMsg := req.Msg

	fnCrudPermission := this.fnCrudPermissionMap[CrudMsg.TableName]

	if fnCrudPermission == nil && CrudMsg.Code != protodb.CrudReqCode_SELECTONE {
		errInfo := fmt.Errorf("no crudpermission function for table %s", CrudMsg.TableName)
		connecterr := connect.NewError(
			connect.CodePermissionDenied,
			errInfo,
		)
		connecterr.Meta().Set("Err-Info", errInfo.Error())

		return nil, connecterr
	}

	respCrud, err := crud.Crud(ctx, meta, CrudMsg, this.FnGetDb, fnCrudPermission)
	if err != nil {
		connecterr := connect.NewError(
			connect.CodeUnknown,
			err,
		)
		connecterr.Meta().Set("Err-Info", err.Error())

		return nil, connecterr
	}

	resp = &connect.Response[protodb.CrudResp]{
		Msg: respCrud,
	}

	return resp, nil

}

func (this *TconnectrpcProtoDbSrvHandlerImpl) TableQuery(ctx context.Context, req *connect.Request[protodb.TableQueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	sendErr := func(err error) error {
		resp := &protodb.QueryResp{
			ResponseNo:  0,
			ErrInfo:     err.Error(),
			MsgBytes:    nil,
			MsgFormat:   0,
			ResponseEnd: true,
		}
		ss.ResponseHeader().Set("Err-Info", err.Error())
		return ss.Send(resp)
	}

	meta := req.Header()
	TableQueryReq := req.Msg

	permissionFn, ok := this.fnTableQueryPermissionMap[TableQueryReq.TableName]
	if !ok {
		return sendErr(fmt.Errorf("no permission check function for table %s", TableQueryReq.TableName))
	}

	fnSend := func(resp *protodb.QueryResp) error {
		if len(resp.ErrInfo) > 0 {
			ss.ResponseHeader().Set("Err-Info", resp.ErrInfo)
		}
		return ss.Send(resp)
	}
	return crud.TableQuery(ctx, meta, TableQueryReq, this.FnGetDb, permissionFn, fnSend)
}

func (this *TconnectrpcProtoDbSrvHandlerImpl) Query(ctx context.Context, req *connect.Request[protodb.QueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	fnSend := func(resp *protodb.QueryResp) error {
		if len(resp.ErrInfo) > 0 {
			ss.ResponseHeader().Set("Err-Info", resp.ErrInfo)
		}
		return ss.Send(resp)
	}
	return crud.Query(ctx, req.Header(), req.Msg, this.FnGetDb, fnSend)

}
