package crud

import (
	"fmt"
	"unicode"
)

// enum of  ColumnNameCheckMethod
// 0: strict, in key column
// 1: in where,or in result column
type ColumnNameCheckMethod int

const (
	ColumnNameCheckMethodStrict ColumnNameCheckMethod = iota
	ColumnNameCheckMethodInWhereOrResult
)

func checkSQLColumnsIsNoInjection(columns []string, checkMethod ColumnNameCheckMethod) (err error) {
	for _, column := range columns {
		switch checkMethod {
		case ColumnNameCheckMethodStrict:
			err = checkSQLColumnsIsNoInjectionStrict(column)
		case ColumnNameCheckMethodInWhereOrResult:
			err = checkSQLColumnsIsNoInjectionInWhere(column)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func checkSQLColumnsIsNoInjectionStrict(columnName string) error {
	if columnName == "*" {
		return nil
	}

	allDigitCount := 0
	for _, c := range columnName {
		if unicode.IsLetter(c) {
			continue
		} else if c == '_' {
			continue
		} else if unicode.IsDigit(c) {
			allDigitCount++
			continue
		} else if c == '(' {
			allDigitCount++
			continue
		} else if c == ')' {
			allDigitCount++
			continue
		} else if c == '.' {
			allDigitCount++
			continue
		}

		return fmt.Errorf("column %s contains invalid character %c", columnName, c)
	}

	if allDigitCount == len(columnName) {
		return fmt.Errorf("column %s contains only digits", columnName)
	}

	return nil
}

func checkSQLColumnsIsNoInjectionInWhere(columnName string) error {
	if columnName == "*" {
		return nil
	}
	for _, c := range columnName {
		if unicode.IsLetter(c) {
			continue
		} else if c == '_' {
			continue
		} else if unicode.IsDigit(c) {
			continue
		} else if c == '(' {
			continue
		} else if c == ')' {
			continue
		} else if c == '[' {
			continue
		} else if c == ']' {
			continue
		} else if c == '\'' {
			continue
		} else if c == ' ' {
			continue
		} else if c == ':' {
			continue
		} else if c == '+' {
			continue
		} else if c == '-' {
			continue
		} else if c == '>' {
			continue
		} else if c == '.' {
			continue
		}

		return fmt.Errorf("column %s contains invalid character %c", columnName, c)
	}
	return nil
}
