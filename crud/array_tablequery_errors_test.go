package crud

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
)

type dummyDB2 struct{}

func (dummyDB2) Exec(query string, args ...any) (sql.Result, error) { return nil, nil }
func (dummyDB2) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}
func (dummyDB2) Query(query string, args ...any) (*sql.Rows, error) { return nil, nil }
func (dummyDB2) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}
func (dummyDB2) QueryRow(query string, args ...any) *sql.Row { return &sql.Row{} }
func (dummyDB2) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return &sql.Row{}
}
func (dummyDB2) Prepare(query string) (*sql.Stmt, error)                             { return nil, nil }
func (dummyDB2) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) { return nil, nil }

func TestTableQueryBuildSql_Where2MismatchLen(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	_, _, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where2: map[string]string{
			"ResultColumnNames": "a",
		},
		Where2Operator: map[string]protodb.WhereOperator{},
	}, "", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTableQueryBuildSql_Where2MissingOperatorForField(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	_, _, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where2: map[string]string{
			"ResultColumnNames": "a",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"TableName": protodb.WhereOperator_WOP_EQ,
		},
	}, "", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTableQueryBuildSql_Where2FieldNotFound(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	_, _, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where2: map[string]string{
			"NoSuchField": "a",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"NoSuchField": protodb.WhereOperator_WOP_EQ,
		},
	}, "", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTableQueryBuildSql_Where2NonListUnsupportedOp(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	_, _, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where2: map[string]string{
			"TableName": "x",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"TableName": protodb.WhereOperator_WOP_CONTAINS,
		},
	}, "", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTableQueryBuildSql_Where2ListUnsupportedOp(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	_, _, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where2: map[string]string{
			"ResultColumnNames": "x",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_GT,
		},
	}, "", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTableQueryBuildSql_ResultColumnsInjectionRejected(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	_, _, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName:         "t",
		ResultColumnNames: []string{"a;drop table t"},
	}, "", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTableQueryBuildSql_PermissionAndWhereClause(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.SQLite}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where: map[string]string{
			"TableName": "abc",
		},
	}, "1=1", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "WHERE") {
		t.Fatalf("expected WHERE, got %s", sqlStr)
	}
	if len(vals) != 1 || vals[0] != "abc" {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_SelectStar_LimitOffset_AndPermissionVals(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Limit:     10,
		Offset:    5,
	}, "(a = $1)", []any{"p"})
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "SELECT") || !strings.Contains(sqlStr, "*") {
		t.Fatalf("expected select *, got %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "LIMIT") || !strings.Contains(sqlStr, "OFFSET") {
		t.Fatalf("expected limit/offset, got %s", sqlStr)
	}
	if len(vals) != 1 || vals[0] != "p" {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_WhereMap_Placeholders_Postgres(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB2{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, &protodb.TableQueryReq{
		TableName: "t",
		Where: map[string]string{
			"TableName":  "a",
			"SchemeName": "b",
		},
	}, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "$1") || !strings.Contains(sqlStr, "$2") {
		t.Fatalf("expected numbered placeholders, got %s", sqlStr)
	}
	if len(vals) != 2 {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestWhereOperator2Str_Coverage(t *testing.T) {
	if strings.TrimSpace(WhereOperator2Str(protodb.WhereOperator_WOP_EQ)) != "=" {
		t.Fatalf("unexpected")
	}
	if strings.TrimSpace(WhereOperator2Str(protodb.WhereOperator_WOP_LIKE)) != "LIKE" {
		t.Fatalf("unexpected")
	}
	if strings.TrimSpace(WhereOperator2Str(protodb.WhereOperator_WOP_GT)) != ">" {
		t.Fatalf("unexpected")
	}
	if strings.TrimSpace(WhereOperator2Str(protodb.WhereOperator_WOP_LT)) != "<" {
		t.Fatalf("unexpected")
	}
	if strings.TrimSpace(WhereOperator2Str(protodb.WhereOperator_WOP_GTE)) != ">=" {
		t.Fatalf("unexpected")
	}
	if strings.TrimSpace(WhereOperator2Str(protodb.WhereOperator_WOP_LTE)) != "<=" {
		t.Fatalf("unexpected")
	}
	_ = WhereOperator2Str(protodb.WhereOperator_WOP_UNKNOWN)
}
