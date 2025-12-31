package crud

import "testing"

// Quick validation test for optimized functions
func TestOptimizedValidation(t *testing.T) {
	// Test validateIdentifier
	t.Run("validateIdentifier", func(t *testing.T) {
		validCases := []string{"username", "user_name", "table.column", "*"}
		for _, tc := range validCases {
			if err := validateIdentifier(tc); err != nil {
				t.Errorf("validateIdentifier(%q) should pass, got error: %v", tc, err)
			}
		}

		invalidCases := []string{"123", "user;name", ""}
		for _, tc := range invalidCases {
			if err := validateIdentifier(tc); err == nil {
				t.Errorf("validateIdentifier(%q) should fail", tc)
			}
		}
	})

	// Test validateExpression
	t.Run("validateExpression", func(t *testing.T) {
		validCases := []string{
			"count(id)",
			"price * 0.5",
			"amount::text",
			"to_char(created_at, 'YYYY-MM-DD')",
			"(a + b) / c",
			"*",
		}
		for _, tc := range validCases {
			if err := validateExpression(tc); err != nil {
				t.Errorf("validateExpression(%q) should pass, got error: %v", tc, err)
			}
		}

		invalidCases := []string{
			"id = 1 OR 1=1",
			"id = 1 or 1=1",
			"(SELECT password FROM users)",
			"username UNION SELECT",
			"pg_sleep(10)",
			"DROP TABLE users",
			"user--name",
			"user/*comment*/",
			"count(id",
			"arr[1",
		}
		for _, tc := range invalidCases {
			if err := validateExpression(tc); err == nil {
				t.Errorf("validateExpression(%q) should fail", tc)
			}
		}
	})
}
