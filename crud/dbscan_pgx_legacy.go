//go:build !go1.27

package crud

import (
	"database/sql"
	"github.com/ygrpc/protodb/sqldb"
)

func enablePGXColumnScanning(_ *sql.Rows, _ *DbRowScanner) error { return nil }

const pgxNativeScanEnabled = false

func pgxScanQueryArgs(_ sqldb.DB, args []any) []any { return args }
