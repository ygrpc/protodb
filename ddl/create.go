package ddl

import (
	"database/sql"
	"fmt"

	"github.com/ygrpc/protodb"
	"github.com/ygrpc/protodb/pdbutil"
	"github.com/ygrpc/protodb/protosql"
	"github.com/ygrpc/protodb/sqldb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func DbCreateSQL(db *sql.DB, msg proto.Message, dbschema string) (sqlStr string, err error) {
	msgPm := msg.ProtoReflect()
	msgDesc := msgPm.Descriptor()
	msgFieldDescs := msgDesc.Fields()
	tableName := string(msgDesc.Name())

	return dbCreateSQL(db, msg, dbschema, tableName, msgDesc, msgFieldDescs)
}

func dbCreateSQL(db *sql.DB, msg proto.Message, dbschema string, tableName string, msgDesc protoreflect.MessageDescriptor, msgFieldDescs protoreflect.FieldDescriptors) (sqlStr string, err error) {
	fieldPdbMap := map[string]*protodb.PDBField{}
	primarykeys := []protoreflect.FieldDescriptor{}

	//uniquename->proto field
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
				uniquekeysMap[pdb.UniqueName] = append(uniquekeysMap[pdb.UniqueName], fieldDesc)
			}
		}
	}

	sqlStr = ""

	pdbm, _ := pdbutil.GetPDBM(msgDesc)
	if pdbm != nil {
		for _, presql := range pdbm.SQLPrepend {
			sqlStr += presql + "\n"
		}

		for _, comment := range pdbm.Comment {
			sqlStr += "-- " + comment + "\n"
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

		if len(fieldPdb.Comment) > 0 {
			for _, comment := range fieldPdb.Comment {
				sqlStr += "-- " + comment + "\n"
			}
		}

		sqlStr += fieldname + " "

		sqlStr += getSqlTypeStr(fieldMsg, fieldPdb)

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
		}

		if len(fieldPdb.DefaultValue) > 0 {
			sqlStr += protosql.DEFAULT + fieldPdb.DefaultValue

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

	return sqlStr, nil

}

func getSqlTypeStr(fieldMsg protoreflect.FieldDescriptor, fieldPdb *protodb.PDBField) string {
	//get db type from pdb
	pdbdbtype := fieldPdb.PdbDbTypeStr(fieldMsg)
	if len(pdbdbtype) > 0 {
		return pdbdbtype
	} else {
		fmt.Println("todo: getSqlTypeStr unknown db type for field:", fieldMsg.FullName(), " using default type text")
		return "text"
	}
}

// DbMigrateTable migrate a table to the definition of proto message
func DbMigrateTable(db *sql.DB, msg proto.Message, dbschema string) (sqlStr []string, err error) {

	return nil, err
}
