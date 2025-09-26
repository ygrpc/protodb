package crud

import (
	"database/sql"
	"errors"
	"strings"
	"unsafe"

	"github.com/puzpuzpuz/xsync/v3"
)

type TpgVersion struct {
	Major int
	Minor int
}

var emptyPgVersion = TpgVersion{}
var errPgVersionNotFound = errors.New("pg version not found")

var pgversionMap = xsync.NewMapOf[unsafe.Pointer, TpgVersion]()

func GetPgVersion(db *sql.DB) (version TpgVersion, err error) {
	if v, ok := pgversionMap.Load(unsafe.Pointer(db)); ok {
		version = v
		return
	} else {
		pgversion, err := SearchPgVersionInDB(db)
		if err != nil {
			return emptyPgVersion, err
		}
		pgversionMap.Store(unsafe.Pointer(db), pgversion)
		return pgversion, nil
	}

}

func SearchPgVersionInDB(db *sql.DB) (version TpgVersion, err error) {
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
