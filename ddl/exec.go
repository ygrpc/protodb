package ddl

import (
	"database/sql"
	"fmt"
)

// ExecSql executes the given SQL statements on the given database.
func ExecSql(db *sql.DB, sqlStats []*TDbTableInitSql) error {
	initialized := make(map[string]struct{})

	for _, stmt := range sqlStats {
		if err := initTable(db, stmt, initialized); err != nil {
			return fmt.Errorf("execSql err %s: %w", stmt.TableName, err)
		}
	}

	return nil
}

func initTable(db *sql.DB, stmt *TDbTableInitSql, initializedTable map[string]struct{}) error {
	// Skip if already initialized
	if _, done := initializedTable[stmt.TableName]; done {
		return nil
	}

	// First initialize dependencies
	for depName, depStmt := range stmt.DepTableSqlItemMap {
		if err := initTable(db, depStmt, initializedTable); err != nil {
			return fmt.Errorf("executing dependency table %s SQL fail, %w", depName, err)
		}
	}

	// Execute this table's statements
	for _, sqlStr := range stmt.SqlStr {
		if _, err := db.Exec(sqlStr); err != nil {
			return fmt.Errorf("executing SQL: %w", err)
		}
	}

	// Mark as initialized
	initializedTable[stmt.TableName] = struct{}{}
	return nil
}
