package crud

import (
	"fmt"
	"unicode"
)

func checkSQLColumnsIsNoInjection(columns []string) error {
	for _, column := range columns {
		err := checkSQLColumnsIsNoInjectionStr(column)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkSQLColumnsIsNoInjectionStr(columnName string) error {
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
