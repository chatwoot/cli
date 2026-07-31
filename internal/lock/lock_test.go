package lock

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAcquireConflictAndRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")

	l1, err := acquireAt(dir, 42)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	if _, err := acquireAt(dir, 42); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire: want ErrLocked, got %v", err)
	}

	// A different conversation is unaffected.
	l2, err := acquireAt(dir, 43)
	if err != nil {
		t.Fatalf("acquire other conversation: %v", err)
	}
	l2.Release()

	l1.Release()
	l3, err := acquireAt(dir, 42)
	if err != nil {
		t.Fatalf("reacquire after release: %v", err)
	}
	l3.Release()
}

func TestReleaseIsIdempotent(t *testing.T) {
	l, err := acquireAt(filepath.Join(t.TempDir(), "locks"), 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	l.Release()
	l.Release() // must not panic
	var nilLock *Lock
	nilLock.Release() // nil receiver must be safe
}
