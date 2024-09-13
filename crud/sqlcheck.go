package crud

import (
	"fmt"
	"unicode"
)

func checkSQLColumnsIsNoInjection(columns []string) error {
	for _, column := range columns {
		if column == "*" {
			continue
		}
		for _, c := range column {
			if unicode.IsLetter(c) {
				continue
			} else if c == '_' {
				continue
			} else if unicode.IsDigit(c) {
				continue
			}

			return fmt.Errorf("column %s contains invalid character %c", column, c)
		}
	}
	return nil
}
