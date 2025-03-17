package crud

import (
	"database/sql"
	"google.golang.org/protobuf/proto"
)

type TqueryItem struct {
	Err   *string
	IsEnd bool
	Msg   proto.Message
}

func DbTableQuery(db *sql.DB, msg proto.Message, where map[string]string, resultColumns []string, schemaName string, tableName string, permissionSqlStr string, resultCh chan TqueryItem) (err error) {

	return nil
}
