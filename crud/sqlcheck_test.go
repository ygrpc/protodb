package crud

import (
	"testing"
)

func TestCheckSQLColumnsIsNoInjection(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		// Valid Identifiers
		{"valid_simple", "username", false},
		{"valid_snake", "user_name", false},
		{"valid_mixed", "UserName", false},
		{"valid_star", "*", false},
		{"valid_dot", "table.column", false},

		// Valid Expressions
		{"valid_func", "count(id)", false},
		{"valid_cast", "amount::text", false},
		{"valid_math", "price * 0.5", false},
		{"valid_format", "to_char(created_at, 'YYYY-MM-DD')", false},
		{"valid_complex_math", "(a + b) / c", false},
		{"valid_digits_expr", "123", false},
		{"valid_bracket", "user[name]", false},
		{"valid_json_arrow", "data->'key'", false},
		{"valid_json_arrow_text", "data->>'key'", false},

		// Invalid Characters
		{"invalid_semicolon", "user;name", true},
		{"invalid_comment_dash", "user--name", true},
		{"invalid_comment_slash", "user/*name*/", true},

		// Injection Attempts
		{"inject_or", "id = 1 OR 1=1", true},
		{"inject_or", "id = 1 or 1=1", true},
		{"inject_select", "(SELECT password FROM users)", true},
		{"inject_union", "username UNION SELECT", true},
		{"inject_sleep", "pg_sleep(10)", true},
		{"inject_drop", "DROP TABLE users", true},

		// Unbalanced
		{"unbalanced_open", "count(id", true},
		{"unbalanced_close", "count)id(", true},
		{"unbalanced_bracket", "arr[1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSQLColumnsIsNoInjectionInWhere(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("input %q expecting error=%v, got %v", tt.input, tt.expectErr, err)
			}
		})
	}
}
