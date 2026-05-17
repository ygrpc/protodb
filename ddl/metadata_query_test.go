package ddl

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestIsPostgresqlTableExistsUsesQueryArgs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(postgresTableExistsSQL)).
		WithArgs("public", "user'; drop table x;--").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := IsPostgresqlTableExists(db, "", "User'; DROP TABLE x;--")
	if err != nil {
		t.Fatalf("IsPostgresqlTableExists: %v", err)
	}
	if !exists {
		t.Fatal("expected table to exist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestIsSQLiteTableExistsUsesQueryArgs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	tableName := "user'; drop table x;--"
	mock.ExpectQuery(regexp.QuoteMeta(sqliteTableExistsSQL)).
		WithArgs(tableName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	exists, err := IsSQLiteTableExists(db, tableName)
	if err != nil {
		t.Fatalf("IsSQLiteTableExists: %v", err)
	}
	if exists {
		t.Fatal("expected table not to exist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetSqliteIndexReturnsRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	rowsErr := errors.New("rows failed")
	mock.ExpectQuery(regexp.QuoteMeta(sqliteIndexInfoSQL)).
		WithArgs("idx").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("id").RowError(0, rowsErr))

	_, err = getSqliteIndex(db, "idx")
	if !errors.Is(err, rowsErr) {
		t.Fatalf("expected rows error %v, got %v", rowsErr, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
