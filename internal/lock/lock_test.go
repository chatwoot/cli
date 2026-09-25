package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireConflictAndRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")

	l1, err := acquireAt(dir, "", 42)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	if _, err := acquireAt(dir, "", 42); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire: want ErrLocked, got %v", err)
	}

	// A different conversation is unaffected.
	l2, err := acquireAt(dir, "", 43)
	if err != nil {
		t.Fatalf("acquire other conversation: %v", err)
	}
	l2.Release()

	l1.Release()
	l3, err := acquireAt(dir, "", 42)
	if err != nil {
		t.Fatalf("reacquire after release: %v", err)
	}
	l3.Release()
}

func TestReleaseIsIdempotent(t *testing.T) {
	l, err := acquireAt(filepath.Join(t.TempDir(), "locks"), "", 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	l.Release()
	l.Release() // must not panic
	var nilLock *Lock
	nilLock.Release() // nil receiver must be safe
}

// The same conversation ID on two accounts is two different conversations.
func TestScopesDoNotConflict(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")

	prod, err := acquireAt(dir, "prod", 42)
	if err != nil {
		t.Fatalf("acquire prod: %v", err)
	}
	defer prod.Release()

	staging, err := acquireAt(dir, "staging", 42)
	if err != nil {
		t.Fatalf("same id on another account must not conflict: %v", err)
	}
	defer staging.Release()

	if _, err := acquireAt(dir, "prod", 42); !errors.Is(err, ErrLocked) {
		t.Fatalf("same scope and id: want ErrLocked, got %v", err)
	}
}

func TestAcquireConversationUsesConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)        // unix
	t.Setenv("USERPROFILE", home) // windows

	l, err := AcquireConversation("abc123", 7)
	if err != nil {
		t.Fatalf("AcquireConversation: %v", err)
	}
	defer l.Release()

	path := filepath.Join(home, ".chatwoot", "locks", "conv-abc123-7.lock")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lock file not at %s: %v", path, err)
	}
	if strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock file = %q, want this process's PID", data)
	}

	if _, err := AcquireConversation("abc123", 7); !errors.Is(err, ErrLocked) {
		t.Fatalf("second AcquireConversation: want ErrLocked, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".chatwoot", "locks", "conv-7.lock")); !os.IsNotExist(err) {
		t.Fatalf("scoped lock also created the unscoped file: %v", err)
	}
}

func TestAcquireAtFailsWhenDirIsAFile(t *testing.T) {
	parent := t.TempDir()
	notADir := filepath.Join(parent, "locks")
	if err := os.WriteFile(notADir, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := acquireAt(notADir, "", 1); err == nil {
		t.Fatal("acquireAt succeeded with a file where the lock dir should be")
	}
}

func TestAcquireAtFailsWhenLockPathIsADirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	if err := os.MkdirAll(filepath.Join(dir, "conv-1.lock"), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if _, err := acquireAt(dir, "", 1); err == nil || errors.Is(err, ErrLocked) {
		t.Fatalf("acquireAt = %v, want an open error (not ErrLocked)", err)
	}
}
