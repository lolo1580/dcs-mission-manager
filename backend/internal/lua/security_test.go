package lua

import (
	"strings"
	"testing"
)

// TestParseRejectsDeepNesting locks the fix for a fatal stack overflow: a file
// with millions of nested tables used to crash the process (Go turns a stack
// overflow into an unrecoverable error).
func TestParseRejectsDeepNesting(t *testing.T) {
	deep := "x = " + strings.Repeat("{", maxNesting+50)
	if _, err := Parse([]byte(deep)); err == nil {
		t.Fatal("excessive nesting must be rejected, not parsed")
	}
}

// TestParseAcceptsReasonableNesting makes sure the guard does not reject real
// DCS data, which nests only a handful deep.
func TestParseAcceptsReasonableNesting(t *testing.T) {
	src := "x = {{ { a = 1 }, { b = 2 } }}"
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("reasonable nesting rejected: %v", err)
	}
	if _, ok := root["x"]; !ok {
		t.Fatal("x missing from parse result")
	}
}

// TestParseRejectsInfinity locks the +Inf guard on numbers.
func TestParseRejectsInfinity(t *testing.T) {
	for _, src := range []string{"x = 1e999", "x = -1e999"} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%q must be rejected (overflows to +-Inf)", src)
		}
	}
}
