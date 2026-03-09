package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const testServiceTableName = "service_contract_test_msg"

var registerServiceTestMsgOnce sync.Once

type fakeDB struct{}

var _ sqldb.DB = fakeDB{}

func (fakeDB) Exec(query string, args ...any) (sql.Result, error) { panic("unexpected Exec call") }
func (fakeDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	panic("unexpected ExecContext call")
}
func (fakeDB) Query(query string, args ...any) (*sql.Rows, error) { panic("unexpected Query call") }
func (fakeDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	panic("unexpected QueryContext call")
}
func (fakeDB) QueryRow(query string, args ...any) *sql.Row { panic("unexpected QueryRow call") }
func (fakeDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	panic("unexpected QueryRowContext call")
}
func (fakeDB) Prepare(query string) (*sql.Stmt, error) { panic("unexpected Prepare call") }
func (fakeDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	panic("unexpected PrepareContext call")
}

func registerServiceTestMsg(t *testing.T) {
	t.Helper()
	registerServiceTestMsgOnce.Do(func() {
		msgstore.RegisterMsg(testServiceTableName, func(new bool) proto.Message {
			return &protodb.PDBField{}
		})
	})
}

func TestHandleCrudUsesJSONMsgFormatBeforePermission(t *testing.T) {
	registerServiceTestMsg(t)

	permissionErr := errors.New("permission denied in test")
	called := false
	msgBytes, err := protojson.Marshal(&protodb.PDBField{
		Comment: []string{"json-ok"},
	})
	if err != nil {
		t.Fatalf("marshal json request msg: %v", err)
	}

	_, err = HandleCrud(
		context.Background(),
		http.Header{},
		&protodb.CrudReq{
			Code:       protodb.CrudReqCode_INSERT,
			ResultType: protodb.CrudResultType_DMLResult,
			TableName:  testServiceTableName,
			MsgBytes:   msgBytes,
			MsgFormat:  1,
		},
		func(meta http.Header, schemaName string, tableName string, writable bool) (sqldb.DB, error) {
			return fakeDB{}, nil
		},
		func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DB, dbmsg proto.Message) error {
			called = true

			field, ok := dbmsg.(*protodb.PDBField)
			if !ok {
				t.Fatalf("unexpected message type %T", dbmsg)
			}
			if got := field.Comment; len(got) != 1 || got[0] != "json-ok" {
				t.Fatalf("json payload was not decoded correctly: %#v", got)
			}
			return permissionErr
		},
	)

	if !called {
		t.Fatal("permission function was not called")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %v", err)
	}
	if connectErr.Code() != connect.CodePermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", connectErr.Code())
	}
}

func TestBuildCrudRespUsesRequestedMsgFormat(t *testing.T) {
	resp, err := buildCrudResp(1, nil, &protodb.PDBField{
		Comment: []string{"json-ok"},
	}, 1)
	if err != nil {
		t.Fatalf("buildCrudResp returned error: %v", err)
	}

	if resp.MsgFormat != 1 {
		t.Fatalf("expected MsgFormat=1, got %d", resp.MsgFormat)
	}

	var decoded protodb.PDBField
	if err := protojson.Unmarshal(resp.NewMsgBytes, &decoded); err != nil {
		t.Fatalf("response is not valid protobuf json: %v", err)
	}
	if got := decoded.Comment; len(got) != 1 || got[0] != "json-ok" {
		t.Fatalf("unexpected decoded payload: %#v", got)
	}
}

func TestCrudPreservesPermissionDeniedCode(t *testing.T) {
	registerServiceTestMsg(t)

	msgBytes, err := proto.Marshal(&protodb.PDBField{})
	if err != nil {
		t.Fatalf("marshal request msg: %v", err)
	}

	srv := NewTconnectrpcProtoDbSrvHandlerImpl(
		func(meta http.Header, schemaName string, tableName string, writable bool) (sqldb.DB, error) {
			return fakeDB{}, nil
		},
		map[string]TfnProtodbCrudPermission{
			testServiceTableName: func(meta http.Header, schemaName string, crudCode protodb.CrudReqCode, db sqldb.DB, dbmsg proto.Message) error {
				return errors.New("permission denied in crud wrapper")
			},
		},
		nil,
	)

	_, err = srv.Crud(context.Background(), connect.NewRequest(&protodb.CrudReq{
		Code:       protodb.CrudReqCode_INSERT,
		ResultType: protodb.CrudResultType_DMLResult,
		TableName:  testServiceTableName,
		MsgBytes:   msgBytes,
		MsgFormat:  0,
	}))
	if err == nil {
		t.Fatal("expected permission error")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %v", err)
	}
	if connectErr.Code() != connect.CodePermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", connectErr.Code())
	}
	if got := connectErr.Meta().Get(YgrpcErr); !strings.Contains(got, "permission denied") {
		t.Fatalf("expected Ygrpc-Err metadata to be set, got %q", got)
	}
}
