package rotate_writer

import (
	"fmt"
	"testing"
)

// RotateError is defined in v2 but never constructed by any v2 code path — it is
// dead code (copied from v1 and not wired up). This test covers the method to keep
// coverage honest, and documents the issue.
func TestRotateError_Error(t *testing.T) {
	re := &RotateError{err: fmt.Errorf("disk full")}
	got := re.Error()
	want := "failed to rotate file: disk full"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}