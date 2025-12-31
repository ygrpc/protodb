package service

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
)

type TconnectrpcProtoDbSrvHandlerImpl struct {
	protodb.UnimplementedProtoDbSrvHandler
	FnGetDb TfnProtodbGetDb

	// proto.message name => fn
	// must set for every protodb message, if no fn for a message, set to nil
	fnCrudPermissionMap map[string]TfnProtodbCrudPermission

	// table name => fn
	// must set for every table, if no fn for a table, set to nil
	fnTableQueryPermissionMap map[string]TfnTableQueryPermission
}

// NewTconnectrpcProtoDbSrvHandlerImpl create new ProtoDbSrvHandler impl in connectrpc
func NewTconnectrpcProtoDbSrvHandlerImpl(fnGetCrudDb TfnProtodbGetDb, fnCrudPermission map[string]TfnProtodbCrudPermission,
	fnTableQueryPermission map[string]TfnTableQueryPermission) *TconnectrpcProtoDbSrvHandlerImpl {
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

	// Secure by Default: deny all operations if no permission function is registered
	if fnCrudPermission == nil {
		errInfo := fmt.Errorf("no crudpermission function registered for table %s, operation %s denied", CrudMsg.TableName, CrudMsg.Code.String())
		connecterr := connect.NewError(
			connect.CodePermissionDenied,
			errInfo,
		)
		connecterr.Meta().Set("Ygrpc-Err", errInfo.Error())

		return nil, connecterr
	}

	respCrud, err := HandleCrud(ctx, meta, CrudMsg, this.FnGetDb, fnCrudPermission)
	if err != nil {
		connecterr := connect.NewError(
			connect.CodeUnknown,
			err,
		)
		connecterr.Meta().Set("Ygrpc-Err", err.Error())

		return nil, connecterr
	}

	resp = &connect.Response[protodb.CrudResp]{
		Msg: respCrud,
	}

	return resp, nil

}

func (this *TconnectrpcProtoDbSrvHandlerImpl) TableQuery(ctx context.Context, req *connect.Request[protodb.TableQueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	meta := req.Header()

	ygrpcErrHeaderStr := meta.Get(YgrpcErrHeader)
	ygrpcErrHeader := len(ygrpcErrHeaderStr) > 0
	ygrpcerrmaxlen := 0
	if ygrpcErrHeader {
		ygrpcerrmax := meta.Get(YgrpcErrMax)
		if len(ygrpcerrmax) > 0 {
			ygrpcerrmaxlen, _ = strconv.Atoi(ygrpcerrmax)
		}
	}

	sendErr := func(err error) error {
		resp := &protodb.QueryResp{
			ResponseNo:  0,
			ErrInfo:     err.Error(),
			MsgBytes:    nil,
			MsgFormat:   0,
			ResponseEnd: true,
		}
		if ygrpcErrHeader {
			errStr := err.Error()
			if ygrpcerrmaxlen > 0 {
				if len(errStr) > ygrpcerrmaxlen {
					errStr = errStr[:ygrpcerrmaxlen]
				}
			}
			ss.ResponseHeader().Set(YgrpcErr, errStr)
		}

		return ss.Send(resp)
	}

	TableQueryReq := req.Msg

	permissionFn, ok := this.fnTableQueryPermissionMap[TableQueryReq.TableName]
	if !ok {
		return sendErr(fmt.Errorf("no permission check function for table %s", TableQueryReq.TableName))
	}

	fnSend := func(resp *protodb.QueryResp) error {
		if len(resp.ErrInfo) > 0 && ygrpcErrHeader {
			errStr := resp.ErrInfo
			if ygrpcerrmaxlen > 0 {
				if len(errStr) > ygrpcerrmaxlen {
					errStr = errStr[:ygrpcerrmaxlen]
				}
			}
			ss.ResponseHeader().Set(YgrpcErr, errStr)
		}
		return ss.Send(resp)
	}
	return HandleTableQuery(ctx, meta, TableQueryReq, this.FnGetDb, permissionFn, fnSend)
}

func (this *TconnectrpcProtoDbSrvHandlerImpl) Query(ctx context.Context, req *connect.Request[protodb.QueryReq], ss *connect.ServerStream[protodb.QueryResp]) error {
	meta := req.Header()

	ygrpcErrHeaderStr := meta.Get(YgrpcErrHeader)
	ygrpcErrHeader := len(ygrpcErrHeaderStr) > 0
	ygrpcerrmaxlen := 0
	if ygrpcErrHeader {
		ygrpcerrmax := meta.Get(YgrpcErrMax)
		if len(ygrpcerrmax) > 0 {
			ygrpcerrmaxlen, _ = strconv.Atoi(ygrpcerrmax)
		}
	}
	fnSend := func(resp *protodb.QueryResp) error {
		if len(resp.ErrInfo) > 0 && ygrpcErrHeader {
			errStr := resp.ErrInfo
			if ygrpcerrmaxlen > 0 {
				if len(errStr) > ygrpcerrmaxlen {
					errStr = errStr[:ygrpcerrmaxlen]
				}
			}
			ss.ResponseHeader().Set(YgrpcErr, errStr)
		}
		return ss.Send(resp)
	}
	return HandleQuery(ctx, req.Header(), req.Msg, this.FnGetDb, fnSend)

}
