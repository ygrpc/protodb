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
	//depend on sql table name in order, value in DepTableSqlItemMap
	DepTableNames      []string
	DepTableSqlItemMap map[string]*TDbTableInitSql
}

func TryAddQuote2DefaultValue(fieldType protoreflect.Kind, defaultValue string) string {
	//if field proto type is number, do not add quote
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
	if strings.HasPrefix(trimmedDefaultValue, "'") || strings.HasSuffix(trimmedDefaultValue, "'") {
		return defaultValue
	}
	//if end with )
	if strings.HasSuffix(trimmedDefaultValue, ")") {
		return defaultValue
	}

	// Escape single quotes by replacing ' with ''
	escapedString := strings.ReplaceAll(defaultValue, "'", "''")

	// Surround the escaped string with single quotes
	return "'" + escapedString + "'"

}

// checkRefference if check reference table
// withComment if add comment for sql
// builtInitSqlMap is a map to store all init sql, user should use a global map for speed up(no need generate init sql again)
func DbCreateSQL(db *sql.DB, msg proto.Message, dbschema string, checkRefference bool, withComment bool, builtInitSqlMap map[string]*TDbTableInitSql) (sqlInitSql *TDbTableInitSql, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	if builtInitSqlMap == nil {
		return nil, errors.New("builtInitSqlMap cannot be nil")
	}

	if sqlInitSql, ok := builtInitSqlMap[tableName]; ok {
		if sqlInitSql == nil {
			return nil, fmt.Errorf("circular reference detected for table %s", tableName)
		}
		return sqlInitSql, nil
	}

	// put nil to mark this table is in process
	builtInitSqlMap[tableName] = nil

	initSqlItem, err := dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs, checkRefference, withComment, builtInitSqlMap)
	builtInitSqlMap[tableName] = initSqlItem
	if err != nil {
		delete(builtInitSqlMap, tableName)
	}
	return initSqlItem, err
}

func dbCreateSQL(db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, checkRefference bool, withComment bool,
	builtInitSqlMap map[string]*TDbTableInitSql) (sqlInitSql *TDbTableInitSql, err error) {
	pdbm, found := pdbutil.GetPDBM(msgDesc)
	if found {
		if pdbm.NotDB {
			return nil, errors.New("do not generate db table for this message by user")
		}
	}

	fieldPdbMap := map[string]*protodb.PDBField{}
	primarykeys := []protoreflect.FieldDescriptor{}

	dbdialect := sqldb.GetDBDialect(db)

	//uniquename->proto field
	uniquekeysMap := map[string][]protoreflect.FieldDescriptor{}
	for i := 0; i < msgFieldDescs.Len(); i++ {
		fieldDesc := msgFieldDescs.Get(i)
		fieldName := string(fieldDesc.Name())

		pdb, _ := pdbutil.GetPDB(fieldDesc)
		fieldPdbMap[fieldName] = pdb

		if pdb.Primary {
			primarykeys = append(primarykeys, fieldDesc)
		}
		if pdb.Unique {
			if len(pdb.UniqueName) > 0 {
				idxName := pdb.UniqueName
				if dbdialect != sqldb.Postgres && len(dbschema) > 0 {
					idxName = dbschema + "_" + idxName
				}
				uniquekeysMap[idxName] = append(uniquekeysMap[idxName], fieldDesc)
			} else {
				idxName := fmt.Sprintf("uk_%s_%s", tableName, fieldName)
				if dbdialect != sqldb.Postgres && len(dbschema) > 0 {
					idxName = fmt.Sprintf("uk_%s_%s_%s", dbschema, tableName, fieldName)
				}
				uniquekeysMap[idxName] = append(uniquekeysMap[idxName], fieldDesc)
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

			if fieldPdb.NotNull {
				sqlStr += protosql.NOT_NULL
			} else {
				sqlStr += protosql.NULL
			}
		}

		if len(fieldPdb.Reference) > 0 {
			sqlStr += protosql.REFERENCES + fieldPdb.Reference
			if checkRefference {
				err := addRefferenceDepSqlForCreate(initSqlItem, fieldPdb.Reference, db, withComment, builtInitSqlMap)
				if err != nil {
					return nil, fmt.Errorf("%s add reference %s fail:%s", tableName, fieldPdb.Reference, err.Error())
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

	for _, s := range pdbm.SQLAppend {
		sqlStr += s + "\n"
	}

	sqlStr += protosql.SQL_RIGHT_PARENTHESES

	for _, s := range pdbm.SQLAppendsAfter {
		sqlStr += s + "\n"
	}

	sqlStr += protosql.SQL_SEMICOLON

	uniqueKeySql := createUniqueKeySql(dbtableName, uniquekeysMap)
	if len(uniqueKeySql) > 0 {
		//new line
		sqlStr += "\n"
		sqlStr += uniqueKeySql
	}

	for _, s := range pdbm.SQLAppendsEnd {
		sqlStr += s + "\n"
	}

	initSqlItem.SqlStr = append(initSqlItem.SqlStr, sqlStr)
	return initSqlItem, nil
}

func createOneUniqueKeySql(tableName string, uniqueName string, uniquekeysFields []protoreflect.FieldDescriptor) string {
	sb := strings.Builder{}

	//sb.WriteString(" create unique index if not exists ")
	sb.WriteString(protosql.SQL_CREATE)
	sb.WriteString(protosql.SQL_UNIQUE)
	sb.WriteString(protosql.SQL_INDEX)
	sb.WriteString(protosql.SQL_IF_NOT_EXISTS)
	sb.WriteString(uniqueName)
	//on
	sb.WriteString(protosql.SQL_ON)
	sb.WriteString(tableName)
	//protosql.SQL_LEFT_PARENTHESES
	sb.WriteString(protosql.SQL_LEFT_PARENTHESES)
	isFirst := true
	for _, field := range uniquekeysFields {
		if isFirst {
			isFirst = false
		} else {
			sb.WriteString(protosql.SQL_COMMA)
		}
		sb.WriteString(string(field.Name()))
	}
	sb.WriteString(protosql.SQL_RIGHT_PARENTHESES)
	sb.WriteString(protosql.SQL_SEMICOLON)
	sb.WriteString("\n")
	return sb.String()
}

func createUniqueKeySql(tableName string, uniquekeysMap map[string][]protoreflect.FieldDescriptor) string {
	sb := strings.Builder{}
	// unique keys
	for uniqueName, uniqueFields := range uniquekeysMap {
		sb.WriteString(createOneUniqueKeySql(tableName, uniqueName, uniqueFields))
	}

	return sb.String()
}

func GetRefTableName(reference string) (string, error) {
	reference = strings.TrimPrefix(reference, " ")
	tablename, _, found := strings.Cut(reference, "(")
	if !found {
		return "", fmt.Errorf("reference is not valid %s", reference)
	}
	return tablename, nil
}

func addRefferenceDepSqlForCreate(item *TDbTableInitSql, reference string, db *sql.DB, withComment bool,
	builtInitSqlMap map[string]*TDbTableInitSql) error {
	refTableName, err := GetRefTableName(reference)
	if err != nil {
		return err
	}

	if item.DepTableSqlItemMap[refTableName] != nil {
		//already add
		return nil
	}

	if item.TableName == refTableName {
		//self reference
		return nil
	}

	depMsg, found := msgstore.GetMsg(refTableName, false)
	if !found {
		return fmt.Errorf("reference table msg %s not found for %s", refTableName, item.TableName)
	}

	depSqlItem, err := DbCreateSQL(db, depMsg, item.DbSchema, true, withComment, builtInitSqlMap)
	if err != nil {
		return err
	}

	item.DepTableSqlItemMap[refTableName] = depSqlItem
	item.DepTableNames = append(item.DepTableNames, refTableName)

	return nil

}

func getSqlTypeStr(fieldMsg protoreflect.FieldDescriptor, fieldPdb *protodb.PDBField, dialect sqldb.TDBDialect) string {
	//get db type from pdb
	pdbdbtype := fieldPdb.PdbDbTypeStr(fieldMsg, dialect)
	if len(pdbdbtype) > 0 {
		return pdbdbtype
	} else {
		fmt.Println("getSqlTypeStr unknown db type for field:", fieldMsg.FullName(), " using default type text")
		return "text"
	}
}

// DbMigrateTable migrate a table to the definition of proto message
// msg can be a proto message
// dbschema can be empty,use for pgsql schema
// checkRefference if check reference table
// withComment if add comment for sql
// builtInitSqlMap is a map to store all init sql, user should use a global map for speed up(no need generate init sql again)
func DbMigrateTable(db *sql.DB, msg proto.Message, dbschema string, checkRefference bool, withComment bool, builtInitSqlMap map[string]*TDbTableInitSql) (migrateItem *TDbTableInitSql, err error) {
	if builtInitSqlMap == nil {
		return nil, errors.New("builtInitSqlMap cannot be nil")
	}

	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	if sqlInitSql, ok := builtInitSqlMap[tableName]; ok {
		if sqlInitSql == nil {
			return nil, fmt.Errorf("circular reference detected for table %s", tableName)
		}
		return sqlInitSql, nil
	}

	// put nil to mark this table is in process
	builtInitSqlMap[tableName] = nil

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
		migrateItem, err = dbMigrateTablePostgres(migrateItem, db, msg, dbschema, tableName, msgDesc, msgFieldDescs, checkRefference, withComment, builtInitSqlMap)
	case sqldb.Mysql:
		migrateItem, err = dbMigrateTableMysql(migrateItem, db, msg, dbschema, tableName, msgDesc, msgFieldDescs, checkRefference, withComment, builtInitSqlMap)
	case sqldb.SQLite:
		migrateItem, err = dbMigrateTableSQLite(migrateItem, db, msg, dbschema, tableName, msgDesc, msgFieldDescs, checkRefference, withComment, builtInitSqlMap)
	default:
		err = fmt.Errorf("not support database dialect %s", dbdialect.String())
	}

	builtInitSqlMap[tableName] = migrateItem
	if err != nil {
		delete(builtInitSqlMap, tableName)
	}

	return migrateItem, err
}

// IsPostgresqlTableExists check if table exists. if dbschema is empty, use public
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

func getPostgresqlIndex(db *sql.DB, schemaName string, idxName string) (indexColumns map[string]struct{}, err error) {
	query := fmt.Sprintf("select indexdef from pg_indexes where schemaname='%s' and indexname='%s' limit 1;", schemaName, idxName)
	var indexdef string
	indexColumns = make(map[string]struct{}, 0)

	err = db.QueryRow(query, schemaName, idxName).Scan(&indexdef)
	if err != nil {
		//if err == sql.ErrNoRows {
		if errors.Is(err, sql.ErrNoRows) {
			return indexColumns, nil
		}
		return indexColumns, err
	}

	//to lower
	indexdef = strings.ToLower(indexdef)
	//cut by (
	_, after, found := strings.Cut(indexdef, "(")
	if !found {
		return
	}
	//cut by )
	before, _, found := strings.Cut(after, ")")
	if !found {
		return
	}
	//split by ,
	for _, column := range strings.Split(before, ",") {
		column = strings.TrimSpace(column)
		columnName := strings.ToLower(column)
		indexColumns[columnName] = struct{}{}
	}

	return indexColumns, nil
}

func dbMigrateTablePostgres(migrateItem *TDbTableInitSql, db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors,
	checkRefference bool, withComment bool, builtInitSqlMap map[string]*TDbTableInitSql) (return_migrateItem *TDbTableInitSql, err error) {
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
		createSQLItem, err := dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs, true, withComment, builtInitSqlMap)

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
	dbdialect := sqldb.Postgres
	dbtableName := sqldb.BuildDbTableName(tableName, dbschema, dbdialect)

	//uniquename->proto field
	uniquekeysMap := map[string][]protoreflect.FieldDescriptor{}

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

		if pdb.Unique {
			if len(pdb.UniqueName) > 0 {
				idxName := pdb.UniqueName

				uniquekeysMap[idxName] = append(uniquekeysMap[idxName], fieldDesc)
			} else {
				idxName := fmt.Sprintf("uk_%s_%s", tableName, fieldName)

				uniquekeysMap[idxName] = append(uniquekeysMap[idxName], fieldDesc)
			}
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

		//check dependency
		if len(pdb.Reference) > 0 && checkRefference {
			depMsgName, err := GetRefTableName(pdb.Reference)
			if err != nil {
				return nil, fmt.Errorf("get reference table name %s err: %s", migrateItem.TableName, err)
			}
			if !isSqlInitItemExist(migrateItem, depMsgName, builtInitSqlMap) {
				depMsg, found := msgstore.GetMsg(depMsgName, false)
				if !found {
					return nil, fmt.Errorf("reference table msg %s not found for %s", pdb.Reference, depMsgName)
				}

				depMigrateItem, err := DbMigrateTable(db, depMsg, dbschema, checkRefference, withComment, builtInitSqlMap)
				if err != nil {
					return nil, fmt.Errorf("%s migrate reference field %s fail:%s", migrateItem.TableName, fieldName, err.Error())
				}
				migrateItem.DepTableSqlItemMap[depMsgName] = depMigrateItem
				migrateItem.DepTableNames = append(migrateItem.DepTableNames, depMsgName)
			}
		}
	}

	if len(alterStatements) > 0 {
		migrateItem.SqlStr = append(migrateItem.SqlStr, alterStatements...)
	}

	//uniqueKeySql := createUniqueKeySql(dbtableName, uniquekeysMap)
	//if len(uniqueKeySql) > 0 {
	//	migrateItem.SqlStr = append(migrateItem.SqlStr, uniqueKeySql)
	//}

	migrateUniqueKeySql := ""
	for uniqueKeyName, uniqueKeyFields := range uniquekeysMap {
		//check if the index exist first,if not create it
		indexColumns, err := getPostgresqlIndex(db, dbschema, uniqueKeyName)
		if err != nil {
			return nil, err
		}
		if len(indexColumns) == 0 {
			//not exist,create it
			createUniqueKeySql := createOneUniqueKeySql(dbtableName, uniqueKeyName, uniqueKeyFields)
			migrateUniqueKeySql += createUniqueKeySql
		} else {
			if !uniqueKeyColumnsEqual(indexColumns, uniqueKeyFields) {
				//not equal,drop it and create it
				dropUniqueKeySql := fmt.Sprintf("drop index if exists %s ;\n", uniqueKeyName)
				if len(dbschema) > 0 {
					dropUniqueKeySql = fmt.Sprintf("drop index if exists %s.%s ;\n", dbschema, uniqueKeyName)
				}
				migrateUniqueKeySql += dropUniqueKeySql
				createUniqueKeySql := createOneUniqueKeySql(dbtableName, uniqueKeyName, uniqueKeyFields)
				migrateUniqueKeySql += createUniqueKeySql
			}
		}
	}
	if len(migrateUniqueKeySql) > 0 {
		migrateItem.SqlStr = append(migrateItem.SqlStr, migrateUniqueKeySql)
	}

	pdbm, found := pdbutil.GetPDBM(msgDesc)
	if found {
		if len(pdbm.SQLMigrate) > 0 {
			migrateItem.SqlStr = append(migrateItem.SqlStr, pdbm.SQLMigrate...)
		}
	}

	return migrateItem, nil
}

func isSqlInitItemExist(item *TDbTableInitSql, tableName string, existTableMap map[string]*TDbTableInitSql) bool {
	if existTableMap == nil {
		existTableMap = make(map[string]*TDbTableInitSql)
	}

	if tableName == item.TableName {
		return true
	}
	if _, ok := existTableMap[tableName]; ok {
		return true
	}

	for _, initSql := range item.DepTableSqlItemMap {
		if isSqlInitItemExist(initSql, tableName, existTableMap) {
			return true
		}
	}

	existTableMap[tableName] = item
	return false
}

func dbMigrateTableMysql(migrateItem *TDbTableInitSql, db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, checkRefference bool, withComment bool,
	builtInitSqlMap map[string]*TDbTableInitSql) (return_migrateItem *TDbTableInitSql, err error) {
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

func getSqliteIndex(db *sql.DB, idxName string) (indexColumns map[string]struct{}, err error) {
	query := fmt.Sprintf("select name from pragma_index_info('%s')", idxName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexColumns = make(map[string]struct{}, 0)

	for rows.Next() {
		var columnName string
		err = rows.Scan(&columnName)
		if err != nil {
			return nil, err
		}
		//to lower
		columnName = strings.ToLower(columnName)
		indexColumns[columnName] = struct{}{}
	}

	return indexColumns, nil
}

func dbMigrateTableSQLite(migrateItem *TDbTableInitSql, db *sql.DB, msg proto.Message, dbschema string, tableName string,
	msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors, checkRefference bool, withComment bool,
	builtInitSqlMap map[string]*TDbTableInitSql) (return_migrateItem *TDbTableInitSql, err error) {
	dbtableName := dbschema + tableName

	// Check if table exists
	exists, err := IsSQLiteTableExists(db, dbtableName)
	if err != nil {
		return nil, fmt.Errorf("error checking table existence: %w", err)
	}

	migrateItem.TableExists = exists

	if !exists {
		// Table doesn't exist, create it
		createSQLItem, err := dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs, true, withComment, builtInitSqlMap)

		if err != nil {
			return nil, err
		}

		return createSQLItem, nil
	}

	//get all columns of table
	query := fmt.Sprintf("SELECT name FROM PRAGMA_TABLE_INFO('%s')", dbtableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error getting table columns: %w", err)
	}

	var existingColumns = make(map[string]bool)
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("error scanning table columns: %w", err)
		}
		//lowercase
		existingColumns[strings.ToLower(columnName)] = true
	}

	rows.Close()

	// Generate ALTER TABLE statements
	var alterStatements []string

	//uniquename->proto field
	uniquekeysMap := map[string][]protoreflect.FieldDescriptor{}

	dbdialect := sqldb.SQLite

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

		if pdb.Unique {
			if len(pdb.UniqueName) > 0 {
				idxName := pdb.UniqueName
				dbdialect := sqldb.GetDBDialect(db)
				if dbdialect != sqldb.Postgres && len(dbschema) > 0 {
					idxName = dbschema + "_" + idxName
				}
				uniquekeysMap[idxName] = append(uniquekeysMap[idxName], fieldDesc)
			} else {
				idxName := fmt.Sprintf("uk_%s_%s", tableName, fieldName)
				if dbdialect != sqldb.Postgres && len(dbschema) > 0 {
					idxName = fmt.Sprintf("uk_%s_%s_%s", dbschema, tableName, fieldName)
				}
				uniquekeysMap[idxName] = append(uniquekeysMap[idxName], fieldDesc)
			}
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

			if len(pdb.DefaultValue) > 0 {
				alterStmt += " DEFAULT " + TryAddQuote2DefaultValue(fieldDesc.Kind(), pdb.DefaultValue)
			}

			if len(pdb.Reference) > 0 {
				alterStmt += " REFERENCES " + pdb.Reference
			}

			alterStmt += ";"
			alterStatements = append(alterStatements, alterStmt)
		}

		//check dependency
		if len(pdb.Reference) > 0 && checkRefference {
			depMsgName, err := GetRefTableName(pdb.Reference)
			if err != nil {
				return nil, fmt.Errorf("get reference table name %s err: %s", migrateItem.TableName, err)
			}
			depMsg, found := msgstore.GetMsg(depMsgName, false)
			if !found {
				return nil, fmt.Errorf("reference table msg %s not found for %s", pdb.Reference, depMsgName)
			}

			depMigrateItem, err := DbMigrateTable(db, depMsg, dbschema, checkRefference, withComment, builtInitSqlMap)
			if err != nil {
				return nil, fmt.Errorf("%s migrate reference field %s fail:%s", migrateItem.TableName, fieldName, err.Error())
			}
			migrateItem.DepTableSqlItemMap[depMsgName] = depMigrateItem
			migrateItem.DepTableNames = append(migrateItem.DepTableNames, depMsgName)
		}
	}

	if len(alterStatements) > 0 {
		migrateItem.SqlStr = append(migrateItem.SqlStr, alterStatements...)
	}

	//migrate unique key index by uniquekeysMap
	//get all unique key index in db
	//check if the index exist first,if not create it
	//if exist,check if the index is unique key,check all field in index is equal to proto message definition

	//uniqueKeySql := createUniqueKeySql(dbtableName, uniquekeysMap)
	//if len(uniqueKeySql) > 0 {
	//	migrateItem.SqlStr = append(migrateItem.SqlStr, uniqueKeySql)
	//}

	migrateUniqueKeySql := ""
	for uniqueKeyName, uniqueKeyFields := range uniquekeysMap {
		//check if the index exist first,if not create it
		indexColumns, err := getSqliteIndex(db, uniqueKeyName)
		if err != nil {
			return nil, err
		}
		if len(indexColumns) == 0 {
			//not exist,create it
			createUniqueKeySql := createOneUniqueKeySql(dbtableName, uniqueKeyName, uniqueKeyFields)
			migrateUniqueKeySql += createUniqueKeySql
		} else {
			if !uniqueKeyColumnsEqual(indexColumns, uniqueKeyFields) {
				//not equal,drop it and create it
				dropUniqueKeySql := fmt.Sprintf("drop index if exists %s;\n", uniqueKeyName)
				migrateUniqueKeySql += dropUniqueKeySql
				createUniqueKeySql := createOneUniqueKeySql(dbtableName, uniqueKeyName, uniqueKeyFields)
				migrateUniqueKeySql += createUniqueKeySql
			}
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

func uniqueKeyColumnsEqual(columns map[string]struct{}, fields []protoreflect.FieldDescriptor) bool {
	if len(columns) != len(fields) {
		return false
	}
	for _, field := range fields {
		fileldName := string(field.Name())
		fileldNameLower := strings.ToLower(fileldName)
		_, ok := columns[fileldNameLower]
		if !ok {
			return false
		}
	}
	return true
}
