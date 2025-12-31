package crud

import (
	"testing"
)

// Benchmark for validateIdentifier
func BenchmarkValidateIdentifier(b *testing.B) {
	testCases := []string{
		"username",
		"user_name",
		"table.column",
		"very_long_identifier_name_with_many_parts",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			validateIdentifier(tc)
		}
	}
}

// Benchmark for validateExpression with simple expressions
func BenchmarkValidateExpressionSimple(b *testing.B) {
	testCases := []string{
		"count(id)",
		"price * 0.5",
		"amount::text",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			validateExpression(tc)
		}
	}
}

// Benchmark for validateExpression with complex expressions
func BenchmarkValidateExpressionComplex(b *testing.B) {
	testCases := []string{
		"to_char(created_at, 'YYYY-MM-DD')",
		"(a + b) / c",
		"data->'key'",
		"CASE WHEN status = 1 THEN 'active' ELSE 'inactive' END",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			validateExpression(tc)
		}
	}
}

// Benchmark for validateExpression with injection attempts (should fail fast)
func BenchmarkValidateExpressionInjection(b *testing.B) {
	testCases := []string{
		"id = 1 OR 1=1",
		"(SELECT password FROM users)",
		"username UNION SELECT",
		"pg_sleep(10)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			validateExpression(tc)
		}
	}
}
