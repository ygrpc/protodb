package sqldb

import (
	"io"
	"log"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetDBPlaceholderCacheStoresItem(t *testing.T) {
	restoreLogOutput := discardLogOutput()
	defer restoreLogOutput()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ClearDBDialectCache(db)

	placeholder, item := GetDBPlaceholderCache(db)
	if item.DB != db {
		t.Fatalf("cached item DB = %p, want %p", item.DB, db)
	}
	if item.Placeholder != placeholder {
		t.Fatalf("cached placeholder = %q, returned placeholder = %q", item.Placeholder, placeholder)
	}
	if item.Dialect.Placeholder() != placeholder {
		t.Fatalf("cached dialect placeholder = %q, returned placeholder = %q", item.Dialect.Placeholder(), placeholder)
	}
	if item.CacheTime.IsZero() {
		t.Fatal("cached item CacheTime is zero")
	}
	dialect, dialectItem := GetDBDialectCache(db)
	if dialectItem.CacheTime != item.CacheTime {
		t.Fatal("dialect cache did not reuse existing placeholder cache item")
	}
	if dialectItem.Dialect != dialect {
		t.Fatalf("cached dialect = %v, returned dialect = %v", dialectItem.Dialect, dialect)
	}
	if GetExecutorDialect(db) != dialect {
		t.Fatalf("executor dialect = %v, want %v", GetExecutorDialect(db), dialect)
	}
	if !ClearDBDialectCache(db) {
		t.Fatal("ClearDBDialectCache returned false after cache miss populated an item")
	}
	if ClearDBDialectCache(db) {
		t.Fatal("ClearDBDialectCache returned true after item was already cleared")
	}
}

func BenchmarkGetDBDialectCacheHit(b *testing.B) {
	restoreLogOutput := discardLogOutput()
	defer restoreLogOutput()

	db, _, err := sqlmock.New()
	if err != nil {
		b.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ClearDBDialectCache(db)
	GetDBDialectCache(db)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetDBDialectCache(db)
	}
}

func BenchmarkGetExecutorDialectSQLDBCacheHit(b *testing.B) {
	restoreLogOutput := discardLogOutput()
	defer restoreLogOutput()

	db, _, err := sqlmock.New()
	if err != nil {
		b.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ClearDBDialectCache(db)
	GetDBDialectCache(db)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetExecutorDialect(db)
	}
}

func discardLogOutput() func() {
	out := log.Writer()
	log.SetOutput(io.Discard)
	return func() {
		log.SetOutput(out)
	}
}
