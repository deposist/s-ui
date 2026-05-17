package common

import "testing"

func TestNewErrorPreservesArgumentSpacingAndTrailingNewline(t *testing.T) {
	err := NewError("unknown action: ", "set")
	if err.Error() != "unknown action:  set\n" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
