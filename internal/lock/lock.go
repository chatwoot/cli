// Package lock provides per-conversation advisory file locks so that
// concurrent chatwoot processes on the same machine don't run mutating
// commands (e.g. reply) against the same conversation at once.
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chatwoot/cli/internal/config"
)

// ErrLocked is returned when another process already holds the lock.
var ErrLocked = errors.New("conversation is locked by another chatwoot process")

// Lock is a held per-conversation lock. Release it with Release.
// The OS releases it automatically if the process exits or crashes.
type Lock struct {
	f *os.File
}

// AcquireConversation takes an exclusive non-blocking lock for the given
// conversation ID. It returns ErrLocked (wrapped) if another process holds it.
func AcquireConversation(id int) (*Lock, error) {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}
	return acquireAt(filepath.Join(cfgDir, "locks"), id)
}

func acquireAt(dir string, id int) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("conv-%d.lock", id))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLock(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("conversation %d: %w", id, err)
	}
	// Best-effort PID marker for debugging; the flock is the real lock.
	_ = f.Truncate(0)
	_, _ = f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
	return &Lock{f: f}, nil
}

// Release unlocks and closes the lock file. The file itself is left in place:
// unlinking it would race with other processes locking the same path.
func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	_ = unlock(l.f)
	_ = l.f.Close()
	l.f = nil
}
