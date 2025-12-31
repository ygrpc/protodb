package crud

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/sqldb"
)

type dummyDB struct{}

func (dummyDB) Exec(query string, args ...any) (sql.Result, error) { return nil, nil }
func (dummyDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}
func (dummyDB) Query(query string, args ...any) (*sql.Rows, error) { return nil, nil }
func (dummyDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}
func (dummyDB) QueryRow(query string, args ...any) *sql.Row { return &sql.Row{} }
func (dummyDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return &sql.Row{}
}
func (dummyDB) Prepare(query string) (*sql.Stmt, error)                             { return nil, nil }
func (dummyDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) { return nil, nil }

func TestTableQueryBuildSql_ArrayContains_Postgres(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "abc",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_CONTAINS,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "ResultColumnNames @> ARRAY[$1]::text[]") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 || vals[0] != "abc" {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_ArrayOverlap_SQLite(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.SQLite}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "[\"a\",\"b\"]",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_OVERLAP,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "json_each(ResultColumnNames)") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 || vals[0] != "[\"a\",\"b\"]" {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_ArrayOverlap_Postgres(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "[\"a\",\"b\"]",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_OVERLAP,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "ResultColumnNames && $1") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 {
		t.Fatalf("unexpected vals: %#v", vals)
	}
	if _, ok := vals[0].([]string); !ok {
		t.Fatalf("expected []string, got %T", vals[0])
	}
}

func TestTableQueryBuildSql_ArrayContainsAll_Postgres(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "[\"a\",\"b\"]",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_CONTAINS_ALL,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "ResultColumnNames @> $1") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 {
		t.Fatalf("unexpected vals: %#v", vals)
	}
	if _, ok := vals[0].([]string); !ok {
		t.Fatalf("expected []string, got %T", vals[0])
	}
}

func TestTableQueryBuildSql_ArrayLenGT_Postgres(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "2",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_LEN_GT,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "cardinality(ResultColumnNames) > $1") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 || !reflect.DeepEqual(vals[0], int64(2)) {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_ArrayLenGTE_Postgres(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.Postgres}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "3",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_LEN_GTE,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "cardinality(ResultColumnNames) >= $1") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 || !reflect.DeepEqual(vals[0], int64(3)) {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_ArrayLenLT_SQLite(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.SQLite}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "4",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_LEN_LT,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "json_array_length(ResultColumnNames) < ?") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 || !reflect.DeepEqual(vals[0], int64(4)) {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}

func TestTableQueryBuildSql_ArrayLenLTE_SQLite(t *testing.T) {
	db := &sqldb.DBWithDialect{Executor: dummyDB{}, Dialect: sqldb.SQLite}
	msgDesc := (&protodb.TableQueryReq{}).ProtoReflect().Descriptor()

	req := &protodb.TableQueryReq{
		SchemeName: "",
		TableName:  "t",
		Where2: map[string]string{
			"ResultColumnNames": "5",
		},
		Where2Operator: map[string]protodb.WhereOperator{
			"ResultColumnNames": protodb.WhereOperator_WOP_LEN_LTE,
		},
	}

	sqlStr, vals, err := TableQueryBuildSql(db, msgDesc, req, "", nil)
	if err != nil {
		t.Fatalf("TableQueryBuildSql: %v", err)
	}
	if !strings.Contains(sqlStr, "json_array_length(ResultColumnNames) <= ?") {
		t.Fatalf("unexpected sql: %s", sqlStr)
	}
	if len(vals) != 1 || !reflect.DeepEqual(vals[0], int64(5)) {
		t.Fatalf("unexpected vals: %#v", vals)
	}
}
