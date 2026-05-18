package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSourceAcquireRejectsMissingPath(t *testing.T) {
	_, cleanup, err := New(filepath.Join(t.TempDir(), "missing.db")).Acquire(context.Background())
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("missing file should fail validation")
	}
}

func TestFileSourceAcquireReturnsExistingPath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(wd, "..", "..", "..", "..", "test-db", "x-ui.db")
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("test-db fixture not available: %v", err)
	}
	path, cleanup, err := New(fixture).Acquire(context.Background())
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	if path != fixture {
		t.Fatalf("unexpected path: %s", path)
	}
}
