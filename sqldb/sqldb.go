package sqldb

import (
	"database/sql"
	"github.com/ygrpc/protodb/protosql"
	"log"
	"reflect"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

type TDBDialect int

const (
	Unknown  TDBDialect = 0
	Postgres TDBDialect = 1
	Mysql    TDBDialect = 2
	SQLite   TDBDialect = 3
)

type TDBDialectCacheItem struct {
	DB          *sql.DB
	Placeholder protosql.SQLPlaceholder
	CacheTime   time.Time
}

var dbDialectCache *xsync.MapOf[*sql.DB, TDBDialectCacheItem] = xsync.NewMapOf[*sql.DB, TDBDialectCacheItem]()

// Placeholder get placeholder of db dialect
func (this TDBDialect) Placeholder() protosql.SQLPlaceholder {
	switch this {
	case Postgres:
		return protosql.SQL_DOLLAR
	case Mysql:
		return protosql.SQL_QUESTION
	case SQLite:
		return protosql.SQL_QUESTION
	}
	return protosql.SQL_QUESTION

}

// GetDBDialect get db dialect of sql.db
func GetDBDialect(db *sql.DB) (dialect TDBDialect) {

	driver := GetDBDriverName(db)

	//todo: compare driver string to known driver names

	log.Printf("driver: %s", driver)

	return Unknown
}

// GetDBPlaceholder get placeholder of sql.db
func GetDBPlaceholder(db *sql.DB) protosql.SQLPlaceholder {
	dialect := GetDBDialect(db)

	return dialect.Placeholder()
}

// GetDBPlaceholder get placeholder of sql.db
func GetDBPlaceholderCache(db *sql.DB) (protosql.SQLPlaceholder, TDBDialectCacheItem) {
	// is in cahce
	if item, ok := dbDialectCache.Load(db); ok {
		return item.Placeholder, item
	}

	dialect := GetDBDialect(db)
	placeholder := dialect.Placeholder()

	item := TDBDialectCacheItem{
		DB:          db,
		Placeholder: placeholder,
		CacheTime:   time.Now(),
	}

	return placeholder, item
}

// clear db dialect cache
func ClearDBDialectCache(db *sql.DB) bool {
	_, ok := dbDialectCache.LoadAndDelete(db)
	return ok
}

// GetDBDriverName get db driver name string of sql.db
func GetDBDriverName(db *sql.DB) string {
	driver := db.Driver()
	return reflect.TypeOf(driver).String()
}
