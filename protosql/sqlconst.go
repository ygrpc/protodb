package protosql

const SQL_LEFT_PARENTHESES = " ( "
const SQL_RIGHT_PARENTHESES = " ) "
const SQL_INSERT_INTO = " INSERT INTO "
const SQL_INSERT_VALUES = " VALUES "
const SQL_UPDATE = " UPDATE "
const SQL_SET = " SET "
const SQL_EQUEAL = " = "

// 
// >
t SQL_GT = " > "

//
<
const SQL_LT = " <
const SQL_GT = " > "
E = 
// <
const SQL_LT = " < "
// >=
const SQL_GTE = " >= "
// <=
const SQL_LTE = " <= "
const SQL_LIKE = " LIKE "
const SQL_COMMA = " , "
const SQL_SEMICOLON = " ; "
const SQL_SPACE = " "
const SQL_DOT = "."
const SQL_DELETE = " DELETE "
const SQL_FROM = " FROM "
const SQL_SELECT = " SELECT "
const SQL_WHERE = " WHERE "
const SQL_AND = " AND "
const SQL_OR = " OR "
const SQL_ANY = " ANY "
const SQL_1E1 = " 1 = 1 "
const SQL_ORDER_BY = " ORDER BY "
const SQL_INTERVAL = " INTERVAL "
const SQL_MINUTE = " MINUTE "
const SQL_MINUTES = " MINUTES "
const SQL_AS = " AS "
const SQL_APOSTROPHE = "'"
const SQL_PLUS = "+"
const SQL_MINUS = "-"
const sql_DATETIME = "datetime"
const SQL_TIMESTAMPADD = "TIMESTAMPADD"
const SQL_LIMIT = " LIMIT "
const SQL_LIMIT_1 = SQL_LIMIT + " 1 "
const SQL_OFFSET = " OFFSET "
const SQL_RETURNING = " RETURNING "
const SQL_ASTERISK = " * "
const SQL_CREATE = "CREATE "

// create table
const SQL_CREATETABLE = SQL_CREATE + "TABLE "

// if not exists
const SQL_IFNOTEXISTS = " IF NOT EXISTS "
const SQL_IF_NOT_EXISTS = SQL_IFNOTEXISTS

// SQL_PRIMARYKEY
const SQL_PRIMARYKEY = " PRIMARY KEY "

// NOT_NULL
const NOT_NULL = " NOT NULL "

// null
const NULL = " NULL "

// UNIQUE
const UNIQUE = " UNIQUE "
const SQL_UNIQUE = UNIQUE

// index
const SQL_INDEX = " INDEX "

// REFERENCES
const REFERENCES = " REFERENCES "

// DEFAULT
const DEFAULT = " DEFAULT "

// ON
const SQL_ON = " ON "

type SQLPlaceholder string

var SQL_QUESTION SQLPlaceholder = "?"

const SQL_DOLLAR SQLPlaceholder = "$"
