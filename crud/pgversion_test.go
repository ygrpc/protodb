package crud

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSearchPgVersionInDB_Integration(t *testing.T) {
	dsn := os.Getenv("PROTO_DB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("skip integration test: set PROTO_DB_TEST_PG_DSN")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db error: %v", err)
	}
	defer db.Close()

	gotVersion, err := SearchPgVersionInDB(db)
	if err != nil {
		t.Fatalf("SearchPgVersionInDB() error = %v", err)
	}
	if gotVersion.Major <= 0 {
		t.Fatalf("invalid pg major version: %+v", gotVersion)
	}
}

func TestExtractPgNumericVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "PostgreSQL 15.3 on x86_64-pc-linux-gnu", want: "15.3"},
		{in: "16beta2", want: "16beta2"},
		{in: "foo bar", want: ""},
	}
	for _, tt := range tests {
		if got := extractPgNumericVersion(tt.in); got != tt.want {
			t.Fatalf("extractPgNumericVersion(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestAtoiPrefix(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "15", want: 15},
		{in: "12beta1", want: 12},
		{in: "x12", want: 0},
		{in: "", want: 0},
	}
	for _, tt := range tests {
		if got := atoiPrefix(tt.in); got != tt.want {
			t.Fatalf("atoiPrefix(%q)=%d want %d", tt.in, got, tt.want)
		}
	}
}
