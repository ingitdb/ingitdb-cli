package dalgo2ingitdb

import (
	"fmt"
)

// fileLocker is the advisory-lock surface used by withSharedLock and
// withExclusiveLock. *flock.Flock satisfies it; tests inject a mock via the
// newFileLocker seam to exercise lock-acquisition failures.
type fileLocker interface {
	Lock() error
	RLock() error
	Unlock() error
}

// withSharedLock acquires a shared (read) advisory lock on the file at
// path, calls fn, then releases the lock. On Unix the lock is provided by
// syscall.Flock; on Windows by LockFileEx. Lock files are the actual
// target files — no sidecar .lock files are created.
//
// Multiple goroutines / processes may hold simultaneous shared locks on
// the same file. An attempt to acquire an exclusive lock while any shared
// lock is held blocks until all shared locks release.
//
// The lock is released even when fn returns an error.
func withSharedLock(path string, fn func() error) error {
	lk := newFileLocker(path)
	if err := lk.RLock(); err != nil {
		return fmt.Errorf("acquire shared lock on %s: %w", path, err)
	}
	defer func() {
		_ = lk.Unlock()
	}()
	return fn()
}

// withExclusiveLock acquires an exclusive (write) advisory lock on the
// file at path, calls fn, then releases the lock. Only one holder may
// have an exclusive lock at a time; the call blocks while any other
// shared or exclusive lock is held.
//
// The lock is released even when fn returns an error.
func withExclusiveLock(path string, fn func() error) error {
	lk := newFileLocker(path)
	if err := lk.Lock(); err != nil {
		return fmt.Errorf("acquire exclusive lock on %s: %w", path, err)
	}
	defer func() {
		_ = lk.Unlock()
	}()
	return fn()
}
