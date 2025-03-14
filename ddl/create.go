package ddl

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/msgstore"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type TDbTableInitSql struct {
	DbSchema    string
	TableName   string
	TableExists bool
	SqlStr      []string
	// depend on sql table name in order, value in DepTableSqlItemMap
	DepTableNames      []string
	DepTableSqlItemMap map[string]*TDbTableInitSql
}

func TryAddQuote2DefaultValue(fieldType protoreflect.Kind, defaultValue string) string {
	// if field proto type is number, do not add quote
	switch fieldType {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		{
			return defaultValue
		}
	}

	trimmedDefaultValue := strings.TrimSpace(defaultValue)
	if strings.HasPrefix(trimmedDefaultValue, "'") ||
		strings.HasSuffix(trimmedDefaultValue, "'") {
		return defaultValue
	}
	// if end with )
	if strings.HasSuffix(trimmedDefaultValue, ")") {
		return defaultValue
	}

	// Escape single quotes by replacing ' with ''
	escapedString := strings.ReplaceAll(defaultValue, "'", "''")

	// Surround the escaped string with single quotes
	return "'" + escapedString + "'"
}

func DbCreateSQL(db *sql.DB, msg proto.Message, dbschema string, checkRefference bool,
	withComment bool,
) (sqlInitSql *TDbTableInitSql, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs, checkRefference, withComment)
}

func dbCreateSQL(db *sql.DB, msg proto.Message, dbschema, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors,
	checkRefference, withComment bool,
) (sqlInitSql *TDbTableInitSql, err error) {
	pdbm, found := pdbutil.GetPDBM(msgDesc)
	if found {
		if pdbm.NotDB {
			return nil, errors.New("do not generate db table for this message by user")
		}
	}

	fieldPdbMap := map[string]*protodb.PDBField{}
	primarykeys := []protoreflect.FieldDescriptor{}

	// uniquename->proto field
	uniquekeysMap := map[string][]protoreflect.FieldDescriptor{}
	for i := 0; i < msgFieldDescs.Len(); i++ {
		fieldDesc := msgFieldDescs.Get(i)
		fieldname := string(fieldDesc.Name())

		pdb, _ := pdbutil.GetPDB(fieldDesc)
		fieldPdbMap[fieldname] = pdb

		if pdb.Primary {
			primarykeys = append(primarykeys, fieldDesc)
		}
		if pdb.Unique {
			if len(pdb.UniqueName) > 0 {
				uniquekeysMap[pdb.UniqueName] = append(
					uniquekeysMap[pdb.UniqueName], fieldDesc)
			}
		}
	}

	initSqlItem := &TDbTableInitSql{
		TableName:          tableName,
		SqlStr:             make([]string, 0),
		DepTableNames:      make([]string, 0),
		DepTableSqlItemMap: make(map[string]*TDbTableInitSql),
	}

	sqlStr := ""

	if pdbm != nil {
		for _, presql := range pdbm.SQLPrepend {
			sqlStr += presql + "\n"
		}

		if withComment {
			for _, comment := range pdbm.Comment {
				sqlStr += "-- " + comment + "\n"
			}
		}
	}

	sqlStr += protosql.SQL_CREATETABLE + protosql.SQL_IFNOTEXISTS

	dbdialect := sqldb.GetDBDialect(db)

	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)
	sqlStr += dbtableName
	sqlStr += protosql.SQL_LEFT_PARENTHESES
	sqlStr += "\n"

	for i := 0; i < msgFieldDescs.Len(); i++ {
		fieldDesc := msgFieldDescs.Get(i)
		fieldname := string(fieldDesc.Name())
		fieldPdb := fieldPdbMap[fieldname]
		fieldMsg := fieldDesc

		if fieldPdb.NotDB {
			continue
		}

		if withComment {
			if len(fieldPdb.Comment) > 0 {
				for _, comment := range fieldPdb.Comment {
					sqlStr += "-- " + comment + "\n"
				}
			}
		}

		sqlStr += fieldname + " "

		sqlStr += getSqlTypeStr(fieldMsg, fieldPdb, dbdialect)

		if fieldPdb.IsPrimary() {
		} else {
			if fieldPdb.Unique && len(fieldPdb.UniqueName) == 0 {
				sqlStr += protosql.UNIQUE
			}

			if fieldPdb.NotNull {
				sqlStr += protosql.NOT_NULL
			} else {
				sqlStr += protosql.NULL
			}
		}

		if len(fieldPdb.Reference) > 0 {
			sqlStr += protosql.REFERENCES + fieldPdb.Reference
			if checkRefference {
				err := addRefferenceDepSqlForCreate(initSqlItem, fieldPdb.Reference, db, withComment)
				if err != nil {
					return nil, err
				}
			}
		}

		if len(fieldPdb.DefaultValue) > 0 {
			sqlStr += protosql.DEFAULT +
				TryAddQuote2DefaultValue(fieldDesc.Kind(), fieldPdb.DefaultValue)
		}

		for _, s := range fieldPdb.SQLAppend {
			sqlStr += s + "\n"
		}

		sqlStr += protosql.SQL_COMMA + "\n"

		for _, s := range fieldPdb.SQLAppendsEnd {
			sqlStr += s + "\n"
		}
	}

	// remove last comma
	commaPos := len(sqlStr) - 1
	for {
		// if is space \r \n tab
		if sqlStr[commaPos] == ' ' || sqlStr[commaPos] == '\r' || sqlStr[commaPos] == '\n' || sqlStr[commaPos] == '\t' {
			commaPos--
			continue
		}
		if sqlStr[commaPos] == ',' {
			sqlStr = sqlStr[:commaPos]
			break
		}
		break
	}

	// primary keys
	sqlStr += protosql.SQL_COMMA + protosql.SQL_PRIMARYKEY + protosql.SQL_LEFT_PARENTHESES
	for _, protofield := range primarykeys {
		fieldname := string(protofield.Name())
		sqlStr += fieldname + ","
	}
	sqlStr = sqlStr[:len(sqlStr)-1] + protosql.SQL_RIGHT_PARENTHESES

	// unique keys
	for _, uniqueFields := range uniquekeysMap {
		sqlStr += protosql.SQL_COMMA + protosql.UNIQUE + protosql.SQL_LEFT_PARENTHESES
		for _, field := range uniqueFields {
			sqlStr += string(field.Name()) + ","
		}
		sqlStr = sqlStr[:len(sqlStr)-1] + protosql.SQL_RIGHT_PARENTHESES
	}

	for _, s := range pdbm.SQLAppend {
		sqlStr += s + "\n"
	}

	sqlStr += protosql.SQL_RIGHT_PARENTHESES

	for _, s := range pdbm.SQLAppendsAfter {
		sqlStr += s + "\n"
	}

	sqlStr += protosql.SQL_SEMICOLON

	for _, s := range pdbm.SQLAppendsEnd {
		sqlStr += s + "\n"
	}

	initSqlItem.SqlStr = append(initSqlItem.SqlStr, sqlStr)
	return initSqlItem, nil
}

func GetRefTableName(reference string) (string, error) {
	reference = strings.TrimPrefix(reference, " ")
	tablename, _, found := strings.Cut(reference, "(")
	if !found {
		return "", fmt.Errorf("reference is not valid %s", reference)
	}
	return tablename, nil
}

func addRefferenceDepSqlForCreate(item *TDbTableInitSql, reference string, db *sql.DB, withComment bool) error {
	refTableName, err := GetRefTableName(reference)
	if err != nil {
		return err
	}

	if item.DepTableSqlItemMap[refTableName] != nil {
		// already add
		return nil
	}

	if item.TableName == refTableName {
		// self reference
		return nil
	}

	depMsg, found := msgstore.GetMsg(refTableName, false)
	if !found {
		return fmt.Errorf("reference table msg %s not found for %s", refTableName, item.TableName)
	}

	msgPm := depMsg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()

	depSqlItem, err := dbCreateSQL(db, depMsg, item.DbSchema, refTableName, msgDesc, msgFieldDescs, true, withComment)
	if err != nil {
		return err
	}

	item.DepTableSqlItemMap[refTableName] = depSqlItem
	item.DepTableNames = append(item.DepTableNames, refTableName)

	return nil
}

func getSqlTypeStr(fieldMsg protoreflect.FieldDescriptor, fieldPdb *protodb.PDBField, dialect sqldb.TDBDialect) string {
	// get db type from pdb
	pdbdbtype := fieldPdb.PdbDbTypeStr(fieldMsg, dialect)
	if len(pdbdbtype) > 0 {
		return pdbdbtype
	} else {
		fmt.Println("todo: getSqlTypeStr unknown db type for field:", fieldMsg.FullName(), " using default type text")
		return "text"
	}
}

// DbMigrateTable migrate a table to the definition of proto message.
func DbMigrateTable(db *sql.DB, msg proto.Message, dbschema string, checkRefference bool,
	withComment bool,
) (migrateItem *TDbTableInitSql, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	// Get database dialect
	dbdialect := sqldb.GetDBDialect(db)

	migrateItem = &TDbTableInitSql{
		DbSchema:           dbschema,
		TableName:          tableName,
		SqlStr:             make([]string, 0),
		DepTableNames:      make([]string, 0),
		DepTableSqlItemMap: make(map[string]*TDbTableInitSql),
	}

	switch dbdialect {
	case sqldb.Postgres:
		migrateItem, err = dbMigrateTablePostgres(migrateItem, db, msg, dbschema, tableName,
			msgDesc, msgFieldDescs, checkRefference, withComment)
	case sqldb.Mysql:
		migrateItem, err = dbMigrateTableMysql(migrateItem, db, msg, dbschema, tableName,
			msgDesc, msgFieldDescs, checkRefference, withComment)
	case sqldb.SQLite:
		migrateItem, err = dbMigrateTableSQLite(migrateItem, db, msg, dbschema, tableName,
			msgDesc, msgFieldDescs, checkRefference, withComment)
	default:
		err = fmt.Errorf("not support database dialect %s", dbdialect.String())
	}

	return migrateItem, err
}

// IsPostgresqlTableExists check if table exists. if dbschema is empty, use public.
func IsPostgresqlTableExists(db *sql.DB, dbschema, tableName string) (bool, error) {
	var exists bool
	if len(dbschema) == 0 {
		dbschema = "public"
	}
	query := fmt.Sprintf("SELECT EXISTS (SELECT table_name FROM information_schema.tables WHERE table_schema = '%s' AND table_name = '%s')",
		strings.ToLower(dbschema), strings.ToLower(tableName))
	err := db.QueryRow(query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking table existence: %w", err)
	}
	return exists, nil
}

func dbMigrateTablePostgres(migrateItem *TDbTableInitSql, db *sql.DB, msg proto.Message,
	dbschema, tableName string, msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors, checkRefference, withComment bool,
) (return_migrateItem *TDbTableInitSql, err error) {
	if len(dbschema) == 0 {
		dbschema = "public"
		migrateItem.DbSchema = dbschema
	}
	// Check if table exists
	exists, err := IsPostgresqlTableExists(db, dbschema, tableName)
	if err != nil {
		return nil, err
	}

	migrateItem.TableExists = exists

	if !exists {
		// Table doesn't exist, create it
		createSQLItem, err := dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs, true, withComment)
		if err != nil {
			return nil, err
		}

		return createSQLItem, nil
	}

	// Get existing columns
	query := fmt.Sprintf("SELECT column_name FROM information_schema.columns WHERE table_schema = '%s' AND table_name = '%s'",
		strings.ToLower(dbschema), strings.ToLower(tableName))
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error getting table columns: %w", err)
	}

	existingColumns := make(map[string]bool)
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("error scanning table columns: %w", err)
		}
		// lowercase
		existingColumns[strings.ToLower(columnName)] = true
	}

	rows.Close()

	// Generate ALTER TABLE statements
	var alterStatements []string

	// Get database dialect
	dbdialect := sqldb.GetDBDialect(db)
	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)

	// Process each field in the proto message
	for i := 0; i < msgFieldDescs.Len(); i++ {
		fieldDesc := msgFieldDescs.Get(i)
		fieldName := string(fieldDesc.Name())
		fieldNameLowercase := strings.ToLower(fieldName)
		pdb, _ := pdbutil.GetPDB(fieldDesc)

		// Skip fields marked as NotDB
		if pdb.NotDB {
			continue
		}

		sqlType := getSqlTypeStr(fieldDesc, pdb, dbdialect)

		// Check if column exists
		if _, exists := existingColumns[fieldNameLowercase]; !exists {
			// Column doesn't exist, add it
			alterStmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
				dbtableName, fieldName, sqlType)

			// Add constraints
			if pdb.NotNull {
				alterStmt += " NOT NULL "
			}
			if pdb.Unique && len(pdb.UniqueName) == 0 {
				alterStmt += " UNIQUE "
			}

			if len(pdb.DefaultValue) > 0 {
				alterStmt += " DEFAULT " +
					TryAddQuote2DefaultValue(fieldDesc.Kind(), pdb.DefaultValue)
			}

			if len(pdb.Reference) > 0 {
				alterStmt += " REFERENCES " + pdb.Reference
			}

			alterStmt += ";"
			alterStatements = append(alterStatements, alterStmt)
		}

		// check dependency
		if len(pdb.Reference) > 0 && checkRefference {
			// todo check dependency and add it to migrateItem
		}

		if len(alterStatements) > 0 {
			migrateItem.SqlStr = append(migrateItem.SqlStr, alterStatements...)
		}
	}

	pdbm, found := pdbutil.GetPDBM(msgDesc)
	if found {
		if len(pdbm.SQLMigrate) > 0 {
			migrateItem.SqlStr = append(migrateItem.SqlStr, pdbm.SQLMigrate...)
		}
	}

	return migrateItem, nil
}

func dbMigrateTableMysql(migrateItem *TDbTableInitSql, db *sql.DB, msg proto.Message,
	dbschema, tableName string, msgDesc protoreflect.MessageDescriptor,
	msgFieldDescs protoreflect.FieldDescriptors, checkRefference, withComment bool,
) (return_migrateItem *TDbTableInitSql, err error) {
	return nil, errors.New("not support database dialect Mysql now")
}

// IsSQLiteTableExists check if table exists.
func IsSQLiteTableExists(db *sql.DB, tableName string) (bool, error) {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS (SELECT name FROM sqlite_master WHERE type='table' AND name='%s')", tableName)
	err := db.QueryRow(query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking table existence: %w", err)
	}
	return exists, nil
}

func dbMigrateTableSQLite(migrateItem *TDbTableInitSql, db *sql.DB, msg proto.Message, dbschema, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors,
	checkRefference, withComment bool,
) (return_migrateItem *TDbTableInitSql, err error) {
	dbtableName := dbschema + tableName

	// Check if table exists
	exists, err := IsSQLiteTableExists(db, dbtableName)
	if err != nil {
		return nil, fmt.Errorf("error checking table existence: %w", err)
	}

	migrateItem.TableExists = exists

	if !exists {
		// Table doesn't exist, create it
		createSQLItem, err := dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs, true, withComment)
		if err != nil {
			return nil, err
		}

		return createSQLItem, nil
	}

	// get all columns of table
	query := fmt.Sprintf("SELECT name FROM PRAGMA_TABLE_INFO('%s')", dbtableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error getting table columns: %w", err)
	}

	existingColumns := make(map[string]bool)
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("error scanning table columns: %w", err)
		}
		// lowercase
		existingColumns[strings.ToLower(columnName)] = true
	}

	rows.Close()

	// Generate ALTER TABLE statements
	var alterStatements []string

	// Process each field in the proto message
	for i := 0; i < msgFieldDescs.Len(); i++ {
		fieldDesc := msgFieldDescs.Get(i)
		fieldName := string(fieldDesc.Name())
		fieldNameLowercase := strings.ToLower(fieldName)
		pdb, _ := pdbutil.GetPDB(fieldDesc)

		// Skip fields marked as NotDB
		if pdb.NotDB {
			continue
		}

		sqlType := getSqlTypeStr(fieldDesc, pdb, sqldb.SQLite)

		// Check if column exists
		if _, exists := existingColumns[fieldNameLowercase]; !exists {
			// Column doesn't exist, add it
			alterStmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
				dbtableName, fieldName, sqlType)

			// Add constraints
			if pdb.NotNull {
				alterStmt += " NOT NULL "
			}
			if pdb.Unique && len(pdb.UniqueName) == 0 {
				alterStmt += " UNIQUE "
			}

			if len(pdb.DefaultValue) > 0 {
				alterStmt += " DEFAULT " + TryAddQuote2DefaultValue(fieldDesc.Kind(), pdb.DefaultValue)
			}

			if len(pdb.Reference) > 0 {
				alterStmt += " REFERENCES " + pdb.Reference
			}

			alterStmt += ";"
			alterStatements = append(alterStatements, alterStmt)
		}

		// check dependency
		if len(pdb.Reference) > 0 && checkRefference {
			// todo check dependency and add it to migrateItem
		}

		if len(alterStatements) > 0 {
			migrateItem.SqlStr = append(migrateItem.SqlStr, alterStatements...)
		}
	}

	pdbm, found := pdbutil.GetPDBM(msgDesc)
	if found {
		if len(pdbm.SQLMigrate) > 0 {
			migrateItem.SqlStr = append(migrateItem.SqlStr, pdbm.SQLMigrate...)
		}
	}

	return migrateItem, nil
}
