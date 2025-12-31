package ddl

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestTryAddQuote2DefaultValue_NumberNoQuote(t *testing.T) {
	if got := TryAddQuote2DefaultValue(protoreflect.Int64Kind, "123"); got != "123" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := TryAddQuote2DefaultValue(protoreflect.BoolKind, "true"); got != "true" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestTryAddQuote2DefaultValue_StringQuoteAndEscape(t *testing.T) {
	if got := TryAddQuote2DefaultValue(protoreflect.StringKind, "abc"); got != "'abc'" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := TryAddQuote2DefaultValue(protoreflect.StringKind, "a'b"); got != "'a''b'" {
		t.Fatalf("unexpected: %q", got)
	}
	// already quoted
	if got := TryAddQuote2DefaultValue(protoreflect.StringKind, "'xyz'"); got != "'xyz'" {
		t.Fatalf("unexpected: %q", got)
	}
	// function-like default
	if got := TryAddQuote2DefaultValue(protoreflect.StringKind, "now()"); got != "now()" {
		t.Fatalf("unexpected: %q", got)
	}
}
