package crud

import (
	"fmt"
	"unicode"
)

// ColumnNameCheckMethod enum of  ColumnNameCheckMethod
// 0: strict, in key column
// 1: in where,or in result column
type ColumnNameCheckMethod int

const (
	ColumnNameCheckMethodStrict ColumnNameCheckMethod = iota
	ColumnNameCheckMethodInWhereOrResult
)

// Lookup table for allowed expression operators - O(1) lookup instead of strings.ContainsRune
var allowedExprChars = [256]bool{}

// Dangerous SQL keywords map for O(1) lookup
var dangerousKeywords = map[string]bool{}

func init() {
	// Initialize lookup table for allowed expression characters
	for _, c := range "()+-*/%:[]><,='" {
		allowedExprChars[c] = true
	}

	// Initialize dangerous keywords map (case-insensitive)
	keywords := []string{
		"select", "union", "from", "join", "update", "delete", "insert",
		"drop", "alter", "truncate", "grant", "revoke", "exec", "execute",
		"prepare", "declare", "or", "and", "sleep", "benchmark", "pg_sleep",
	}
	for _, kw := range keywords {
		dangerousKeywords[kw] = true
	}
}

func checkSQLColumnsIsNoInjection(columns []string, checkMethod ColumnNameCheckMethod) (err error) {
	for _, column := range columns {
		switch checkMethod {
		case ColumnNameCheckMethodStrict:
			err = checkSQLColumnsIsNoInjectionStrict(column)
		case ColumnNameCheckMethodInWhereOrResult:
			// Allow expressions in Where Keys and Result Columns
			err = checkSQLColumnsIsNoInjectionInWhere(column)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func checkSQLColumnsIsNoInjectionStrict(columnName string) error {
	// Strict identifiers for table names or critical fields
	return validateIdentifier(columnName)
}

func checkSQLColumnsIsNoInjectionInWhere(columnName string) error {
	// Expressions allowed for calculations and formatting
	return validateExpression(columnName)
}

// Strict identifier (alphanumeric, _, .)
// Optimized: single pass with fast ASCII path
func validateIdentifier(name string) error {
	if name == "*" {
		return nil
	}
	if len(name) == 0 {
		return fmt.Errorf("identifier empty")
	}

	isAllDigits := true
	for _, c := range name {
		// Fast path for ASCII characters
		if c < 128 {
			// ASCII letter: A-Z (65-90) or a-z (97-122)
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '.' {
				isAllDigits = false
				continue
			}
			// ASCII digit: 0-9 (48-57)
			if c >= '0' && c <= '9' {
				continue
			}
			// Invalid ASCII character
			return fmt.Errorf("identifier %s contains invalid character %c", name, c)
		}

		// Slow path for non-ASCII (Unicode) characters
		if unicode.IsLetter(c) {
			isAllDigits = false
		} else if !unicode.IsDigit(c) {
			return fmt.Errorf("identifier %s contains invalid character %c", name, c)
		}
	}

	if isAllDigits {
		return fmt.Errorf("identifier %s contains only digits", name)
	}

	return nil
}

// toLowerCase converts ASCII character to lowercase
func toLowerCase(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// Relaxed validation for Expressions
// Optimized: Complete single-pass validation
// 1. Character whitelist + comment detection
// 2. Parentheses and brackets balance
// 3. O(1) character lookup for operators
// 4. Fast ASCII path to avoid unicode function calls
// 5. Keyword detection in the same loop (no regex)
func validateExpression(expr string) error {
	if expr == "*" {
		return nil
	}
	if len(expr) == 0 {
		return fmt.Errorf("expression empty")
	}

	// Single-pass validation: all checks in one loop
	parenBalance := 0
	bracketBalance := 0
	prevChar := rune(0)

	// For keyword detection
	var currentWord []rune
	inWord := false

	for _, c := range expr {
		// Check for comment patterns (-- and /*)
		if c == '-' && prevChar == '-' {
			return fmt.Errorf("expression contains comment identifier '--'")
		}
		if c == '*' && prevChar == '/' {
			return fmt.Errorf("expression contains comment identifier '/*'")
		}

		// Track parentheses balance
		if c == '(' {
			parenBalance++
		} else if c == ')' {
			parenBalance--
			if parenBalance < 0 {
				return fmt.Errorf("expression has unbalanced parentheses")
			}
		}

		// Track bracket balance
		if c == '[' {
			bracketBalance++
		} else if c == ']' {
			bracketBalance--
			if bracketBalance < 0 {
				return fmt.Errorf("expression has unbalanced brackets")
			}
		}

		// Keyword detection: build words and check
		isLetter := false
		if c < 128 {
			isLetter = (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
		} else {
			isLetter = unicode.IsLetter(c)
		}

		if isLetter {
			if !inWord {
				currentWord = currentWord[:0] // Reset word
				inWord = true
			}
			currentWord = append(currentWord, toLowerCase(c))
		} else {
			// End of word - check if it's dangerous
			if inWord && len(currentWord) > 0 {
				if dangerousKeywords[string(currentWord)] {
					return fmt.Errorf("expression contains dangerous keyword '%s'", string(currentWord))
				}
				inWord = false
			}
		}

		// Character whitelist check with fast ASCII path
		if c < 128 {
			// Fast path for ASCII characters
			// ASCII letter: A-Z (65-90) or a-z (97-122)
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '.' || c == ' ' {
				prevChar = c
				continue
			}
			// ASCII digit: 0-9 (48-57)
			if c >= '0' && c <= '9' {
				prevChar = c
				continue
			}
			// Use lookup table for operators (O(1))
			if allowedExprChars[c] {
				prevChar = c
				continue
			}
			// Invalid ASCII character
			return fmt.Errorf("expression contains invalid character '%c'", c)
		}

		// Slow path for non-ASCII (Unicode) characters
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			prevChar = c
			continue
		}

		return fmt.Errorf("expression contains invalid character '%c'", c)
	}

	// Check last word if any
	if inWord && len(currentWord) > 0 {
		if dangerousKeywords[string(currentWord)] {
			return fmt.Errorf("expression contains dangerous keyword '%s'", string(currentWord))
		}
	}

	// Final balance checks
	if parenBalance != 0 {
		return fmt.Errorf("expression has unbalanced parentheses")
	}
	if bracketBalance != 0 {
		return fmt.Errorf("expression has unbalanced brackets")
	}

	return nil
}
