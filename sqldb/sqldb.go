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
	Oracle   TDBDialect = 4
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

	switch driver {
	case "*stdlib.Driver":
		//pgx stdlib driver,
		return Postgres
	case "*pq.Driver":
		//lib/pq driver
		return Postgres
	case "*sqlite3.SQLiteDriver":
		//mattn/go-sqlite3 driver
		return SQLite
	case "*sqlite.Driver":
		//modernc.org/sqlite driver
		return SQLite
	}
	//todo: compare driver string to known driver names

	log.Printf("not know db dialect for driver: %s", driver)

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

// BuildDbTableName build db table name
func BuildDbTableName(tableName string, dbschema string, dbdialect TDBDialect) string {
	dbtableName := tableName
	if len(dbschema) == 0 {
		//use default table name
	} else {
		switch dbdialect {
		case Postgres, Oracle:
			dbtableName = dbschema + "." + tableName
		default:
			dbtableName = dbschema + tableName
		}
	}
	return dbtableName
}
