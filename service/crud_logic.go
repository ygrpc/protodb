package service

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/crud"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/querystore"
	"google.golang.org/protobuf/proto"
)

func HandleCrud(ctx context.Context, meta http.Header, req *protodb.CrudReq, fnGetDb TfnProtodbGetDb, fnCrudPermission TfnProtodbCrudPermission) (resp *protodb.CrudResp, err error) {

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
			dmlResult, err := crud.DbInsert(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("insert msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult
			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbInsertReturn(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("insert msg %s err: %w", req.TableName, err)
			}
			newMsgBytes, err := crud.MsgMarshal(newMsg, req.MsgFormat)
			if err != nil {
				return nil, fmt.Errorf("marshal msg %s err: %w", req.TableName, err)
			}
			resp = &protodb.CrudResp{
				RowsAffected: 1,
				NewMsgBytes:  newMsgBytes,
				MsgFormat:    req.MsgFormat,
			}
			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		}
	case protodb.CrudReqCode_UPDATE:
		switch req.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := crud.DbUpdate(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("update msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult

			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)

			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbUpdateReturnNew(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
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

			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		case protodb.CrudResultType_OldMsgAndNewMsg:
			oldMsg, newMsg, err := crud.DbUpdateReturnOldAndNew(db, dbmsg, req.MsgLastFieldNo, req.SchemeName)
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
			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		}
	case protodb.CrudReqCode_PARTIALUPDATE:
		switch req.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := crud.DbUpdatePartial(db, dbmsg, req.PartialUpdateFields, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("partialupdate msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult

			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)

			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbUpdatePartialReturnNew(db, dbmsg, req.PartialUpdateFields, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("partialupdate msg %s err: %w", req.TableName, err)
			}
			NewMsgBytes, err := proto.Marshal(newMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal new msg %s err: %w", req.TableName, err)
			}
			resp = &protodb.CrudResp{
				RowsAffected: 1,
				NewMsgBytes:  NewMsgBytes,
			}

			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		case protodb.CrudResultType_OldMsgAndNewMsg:
			oldMsg, newMsg, err := crud.DbUpdatePartialReturnOldAndNew(db, dbmsg, req.PartialUpdateFields, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("partialupdate msg %s err: %w", req.TableName, err)
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

			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		}
	case protodb.CrudReqCode_DELETE:
		switch req.ResultType {
		case protodb.CrudResultType_DMLResult:
			dmlResult, err := crud.DbDelete(db, dbmsg, req.SchemeName)
			if err != nil {
				return nil, fmt.Errorf("delete msg %s err: %w", req.TableName, err)
			}
			resp = dmlResult

			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		case protodb.CrudResultType_NewMsg:
			newMsg, err := crud.DbDeleteReturn(db, dbmsg, req.SchemeName)
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
			go GlobalCrudBroadcaster.Broadcast(meta, db, req, dbmsg, resp)
			return resp, nil
		}
	case protodb.CrudReqCode_SELECTONE:
		newMsg, err := crud.DbSelectOne(db, dbmsg, req.SelectOneKeyFields, req.SelectResultFields, req.SchemeName, true)
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

func HandleTableQuery(ctx context.Context, meta http.Header, req *protodb.TableQueryReq, fnGetDb TfnProtodbGetDb, fnTableQueryPermission TfnTableQueryPermission, fnSend TfnSendQueryResp) (err error) {
	sendErr := func(err error) error {
		resp := &protodb.QueryResp{
			ResponseNo:  0,
			ErrInfo:     err.Error(),
			MsgBytes:    nil,
			MsgFormat:   0,
			ResponseEnd: true,
		}
		return fnSend(resp)
	}

	TableQueryReq := req

	db, err := fnGetDb(meta, TableQueryReq.SchemeName, TableQueryReq.TableName, false)
	if err != nil {
		return sendErr(err)
	}

	dbmsg, ok := msgstore.GetMsg(TableQueryReq.TableName, false)
	if !ok {
		return sendErr(fmt.Errorf("can not get protodb msg %s err", TableQueryReq.TableName))
	}

	permissionSqlStr := ""
	permissionSqlVals := []any{}

	if fnTableQueryPermission != nil {
		permissionSqlStr, permissionSqlVals, err = fnTableQueryPermission(meta, TableQueryReq.SchemeName, TableQueryReq.TableName, db, dbmsg)

		if err != nil {
			return sendErr(fmt.Errorf("permission check for table %s err: %w", TableQueryReq.TableName, err))
		}
	}

	sqlStr, sqlVals, err := crud.TableQueryBuildSql(db, TableQueryReq, permissionSqlStr, permissionSqlVals)

	if err != nil {
		return sendErr(fmt.Errorf("build query sql for %s err: %w", TableQueryReq.TableName, err))
	}

	rows, err := db.Query(sqlStr, sqlVals...)
	if err != nil {
		return sendErr(fmt.Errorf("tablequery %s err: %w", TableQueryReq.TableName, err))
	}
	defer rows.Close()

	// Determine which fields to scan
	resultColumns := TableQueryReq.ResultColumnNames
	useAllFields := len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*")
	fieldNames := resultColumns
	if useAllFields {
		fieldNames = nil
	}

	resultMsg, _ := msgstore.GetMsg(TableQueryReq.TableName, false)

	msgDesc := resultMsg.ProtoReflect().Descriptor()
	msgFieldsMap := pdbutil.BuildMsgFieldsMap(fieldNames, msgDesc.Fields(), true)

	var respNo int64 = 0
	batchSize := TableQueryReq.PreferBatchSize
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
		MsgFormat:   TableQueryReq.MsgFormat,
		MsgBytes:    nil,
		ResponseEnd: false,
	}

	for rows.Next() {
		resultMsg, _ := msgstore.GetMsg(TableQueryReq.TableName, true)

		// Scan row data
		err = crud.DbScan2ProtoMsg(rows, resultMsg,
			fieldNames,
			msgFieldsMap, // msgFieldsMap will be built inside DbScan2ProtoMsg if nil
		)
		if err != nil {
			return sendErr(fmt.Errorf("tablequery %s scan row data err: %w", TableQueryReq.TableName, err))
		}

		resultMsgBytes, err := crud.MsgMarshal(resultMsg, TableQueryReq.MsgFormat)
		if err != nil {
			return sendErr(fmt.Errorf("tablequery %s marshal msg err: %w", TableQueryReq.TableName, err))
		}

		resp.MsgBytes = append(resp.MsgBytes, resultMsgBytes)
		respMsgByteSize += len(resultMsgBytes)
		respBatchSize++

		if respMsgByteSize >= maxMsgByteSize || respBatchSize >= batchSize {
			resp.ResponseNo = respNo
			resp.ResponseEnd = false
			err = fnSend(resp)
			if err != nil {
				return sendErr(fmt.Errorf("send msg fail, %w", err))
			}
			respNo++
			respBatchSize = 0
			respMsgByteSize = 0
			resp = &protodb.QueryResp{
				ResponseNo:  respNo,
				MsgFormat:   TableQueryReq.MsgFormat,
				MsgBytes:    nil,
				ResponseEnd: false,
			}
		}
	}

	err = rows.Err()
	if err != nil {
		return sendErr(fmt.Errorf("query %s err: %w", TableQueryReq.TableName, err))
	}

	resp.ResponseEnd = true
	err = fnSend(resp)
	if err != nil {
		return sendErr(fmt.Errorf("send msg fail, %w", err))
	}

	return nil
}

func HandleQuery(ctx context.Context, meta http.Header, req *protodb.QueryReq, fnGetDb TfnProtodbGetDb, fnSend TfnSendQueryResp) error {

	sendErr := func(err error) error {
		resp := &protodb.QueryResp{
			ResponseNo:  0,
			ErrInfo:     err.Error(),
			MsgBytes:    nil,
			MsgFormat:   0,
			ResponseEnd: true,
		}
		return fnSend(resp)
	}

	executor, err := fnGetDb(meta, "", "", false)
	if err != nil {
		return sendErr(err)
	}

	fn, ok := querystore.GetQuery(req.QueryName)
	if !ok {
		return sendErr(fmt.Errorf("err: can not get query fn for %s", req.QueryName))
	}

	sqlStr, sqlVals, fnGetResultMsg, err := fn(meta, executor, req)
	if err != nil {
		return sendErr(fmt.Errorf("generate query sql for %s err: %w", req.QueryName, err))
	}

	var resultMsg proto.Message

	resultMsg = fnGetResultMsg(false)

	// Determine which fields to scan
	resultColumns := req.ResultColumnNames
	useAllFields := len(resultColumns) == 0 || (len(resultColumns) == 1 && resultColumns[0] == "*")
	fieldNames := resultColumns
	if useAllFields {
		fieldNames = nil
	}

	msgDesc := resultMsg.ProtoReflect().Descriptor()
	msgFieldsMap := pdbutil.BuildMsgFieldsMap(fieldNames, msgDesc.Fields(), true)

	rows, err := executor.Query(sqlStr, sqlVals...)
	if err != nil {
		return sendErr(fmt.Errorf("query %s err: %w", req.QueryName, err))
	}

	defer rows.Close()

	var respNo int64 = 0
	batchSize := req.PreferBatchSize
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
		MsgFormat:   req.MsgFormat,
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

		resultMsgBytes, err := crud.MsgMarshal(resultMsg, req.MsgFormat)
		if err != nil {
			return sendErr(fmt.Errorf("marshal msg err: %w", err))
		}
		resp.MsgBytes = append(resp.MsgBytes, resultMsgBytes)
		respMsgByteSize += len(resultMsgBytes)
		respBatchSize++
		if respMsgByteSize >= maxMsgByteSize || respBatchSize >= batchSize {
			resp.ResponseNo = respNo
			resp.ResponseEnd = false
			err = fnSend(resp)
			if err != nil {
				return sendErr(fmt.Errorf("send msg fail, %w", err))
			}
			respNo++
			respBatchSize = 0
			respMsgByteSize = 0
			resp = &protodb.QueryResp{
				ResponseNo:  respNo,
				MsgFormat:   req.MsgFormat,
				MsgBytes:    nil,
				ResponseEnd: false,
			}
		}
	}

	err = rows.Err()
	if err != nil {
		return sendErr(fmt.Errorf("query %s err: %w", req.QueryName, err))
	}

	resp.ResponseEnd = true
	err = fnSend(resp)
	if err != nil {
		return sendErr(fmt.Errorf("send msg fail, %w", err))
	}
	return nil

}
