package crud

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ygrpc/protodb"
	"google.golang.org/protobuf/proto"
)

func TestDbTableQueryListScansMultipleRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT SchemeName\s*,\s*TableName\s+FROM TableQueryReq\s+WHERE TableName = \?\s+LIMIT 2`).WithArgs("users").WillReturnRows(
		sqlmock.NewRows([]string{"SchemeName", "TableName"}).AddRow("public", "users").AddRow("private", "users"),
	)

	rows, err := DbTableQueryList(db, &protodb.TableQueryReq{}, &protodb.TableQueryReq{
		TableName:         "TableQueryReq",
		ResultColumnNames: []string{"SchemeName", "TableName"},
		Where:             map[string]string{"TableName": "users"},
		Limit:             2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].(*protodb.TableQueryReq).GetSchemeName() != "public" || rows[1].(*protodb.TableQueryReq).GetSchemeName() != "private" {
		t.Fatalf("unexpected rows: %v, %v", rows[0], rows[1])
	}
	if rows[0] == rows[1] {
		t.Fatal("rows must be distinct message instances")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDbTableQueryContextEmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM TableQueryReq WHERE TableName = ?")).WithArgs("missing").WillReturnRows(
		sqlmock.NewRows([]string{"SchemeName", "TableName", "ResultColumnNames", "Where", "Limit", "Offset", "PreferBatchSize", "MsgFormat", "Where2Operator", "Where2"}),
	)
	count := 0
	err = DbTableQueryContext(context.Background(), db, &protodb.TableQueryReq{}, &protodb.TableQueryReq{
		TableName: "TableQueryReq",
		Where:     map[string]string{"TableName": "missing"},
	}, func(proto.Message) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("callback count = %d, want 0", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
