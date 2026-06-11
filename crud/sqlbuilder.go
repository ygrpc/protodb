package crud

import "github.com/ygrpc/protodb/protosql"

func sqlPlaceholderCap(placeholder protosql.SQLPlaceholder, paraNo int) int {
	if placeholder == protosql.SQL_QUESTION {
		return len(protosql.SQL_QUESTION)
	}
	return len(protosql.SQL_DOLLAR) + decimalDigitCount(paraNo)
}

func decimalDigitCount(n int) int {
	return decimalDigitCount64(int64(n))
}

func decimalDigitCount64(n int64) int {
	if n < 0 {
		n = -n
	}
	digits := 1
	for n >= 10 {
		n /= 10
		digits++
	}
	return digits
}
