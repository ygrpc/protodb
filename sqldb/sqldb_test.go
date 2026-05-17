package sqldb

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetDBPlaceholderCacheStoresItem(t *testing.T) {
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
	if item.CacheTime.IsZero() {
		t.Fatal("cached item CacheTime is zero")
	}
	if !ClearDBDialectCache(db) {
		t.Fatal("ClearDBDialectCache returned false after cache miss populated an item")
	}
	if ClearDBDialectCache(db) {
		t.Fatal("ClearDBDialectCache returned true after item was already cleared")
	}
}
