package crud

import (
	"database/sql"
	"errors"
	"strings"
	"unsafe"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/ygrpc/protodb/sqldb"
)

type TpgVersion struct {
	Major int
	Minor int
}

var emptyPgVersion = TpgVersion{}
var errPgVersionNotFound = errors.New("pg version not found")

var pgversionMap = xsync.NewMapOf[unsafe.Pointer, TpgVersion]()

// GetPgVersion gets the PostgreSQL version from cache or database.
// db can be *sql.DB, *sql.Tx or sqldb.DB for transaction support.
// For caching purposes, if db is *sql.DB or *sqldb.DBWithDialect containing *sql.DB,
// the result is cached. For *sql.Tx, the result is not cached.
func GetPgVersion(db sqldb.DB) (version TpgVersion, err error) {
	// Try to get cache key from underlying *sql.DB
	var cacheKey unsafe.Pointer
	switch d := db.(type) {
	case *sql.DB:
		cacheKey = unsafe.Pointer(d)
	case *sqldb.DBWithDialect:
		if innerDB, ok := d.Executor.(*sql.DB); ok {
			cacheKey = unsafe.Pointer(innerDB)
		}
	}

	// Check cache if we have a valid cache key
	if cacheKey != nil {
		if v, ok := pgversionMap.Load(cacheKey); ok {
			return v, nil
		}
	}

	// Query the database for version
	pgversion, err := SearchPgVersionInDB(db)
	if err != nil {
		return emptyPgVersion, err
	}

	// Cache the result if we have a valid cache key
	if cacheKey != nil {
		pgversionMap.Store(cacheKey, pgversion)
	}

	return pgversion, nil
}

// SearchPgVersionInDB queries the database to get the PostgreSQL version.
// db can be *sql.DB, *sql.Tx or sqldb.DB for transaction support.
func SearchPgVersionInDB(db sqldb.DB) (version TpgVersion, err error) {
	// Try multiple ways to get version string
	var verStr string

	// 1) SHOW server_version
	if err = db.QueryRow("SHOW server_version").Scan(&verStr); err != nil {
		// 2) SELECT current_setting('server_version')
		if err2 := db.QueryRow("SELECT current_setting('server_version')").Scan(&verStr); err2 != nil {
			// 3) SELECT version()
			var verFull string
			if err3 := db.QueryRow("SELECT version()").Scan(&verFull); err3 != nil {
				return emptyPgVersion, err
			}
			verStr = extractPgNumericVersion(verFull)
			if verStr == "" {
				return emptyPgVersion, errPgVersionNotFound
			}
		}
	}

	// Parse major.minor from verStr (e.g., "15.3", "14.10 (Debian)")
	major, minor := 0, 0
	// pick first token and split by '.'
	tok := verStr
	if idx := strings.IndexAny(tok, " \t"); idx > 0 {
		tok = tok[:idx]
	}
	parts := strings.SplitN(tok, ".", 3)
	if len(parts) > 0 {
		major = atoiPrefix(parts[0])
	}
	if len(parts) > 1 {
		minor = atoiPrefix(parts[1])
	}

	return TpgVersion{Major: major, Minor: minor}, nil
}

// extractPgNumericVersion extracts a numeric token like "15.3" from a full version string
// e.g., "PostgreSQL 15.3 on x86_64-pc-linux-gnu" -> "15.3"
func extractPgNumericVersion(s string) string {
	for _, f := range strings.Fields(s) {
		if len(f) == 0 {
			continue
		}
		// token starting with digit is likely the version token (e.g., 15.3)
		if f[0] >= '0' && f[0] <= '9' {
			return f
		}
	}
	return ""
}

// atoiPrefix converts the leading digit run of s to int (stops at first non-digit)
func atoiPrefix(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
