package dalgo2ingitdb

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newLockTarget(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed lock target: %v", err)
	}
	return path
}

func TestFileLock_SharedAllowsConcurrent(t *testing.T) {
	t.Parallel()
	path := newLockTarget(t)

	var counter int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Go(func() {
			err := withSharedLock(path, func() error {
				cur := atomic.AddInt32(&counter, 1)
				for {
					seen := atomic.LoadInt32(&maxConcurrent)
					if cur <= seen || atomic.CompareAndSwapInt32(&maxConcurrent, seen, cur) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&counter, -1)
				return nil
			})
			if err != nil {
				t.Errorf("withSharedLock: %v", err)
			}
		})
	}
	wg.Wait()

	if got := atomic.LoadInt32(&maxConcurrent); got < 2 {
		t.Errorf("shared locks should be concurrent: max concurrent observed = %d", got)
	}
}

func TestFileLock_ExclusiveSerializes(t *testing.T) {
	t.Parallel()
	path := newLockTarget(t)

	var counter int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Go(func() {
			err := withExclusiveLock(path, func() error {
				cur := atomic.AddInt32(&counter, 1)
				for {
					seen := atomic.LoadInt32(&maxConcurrent)
					if cur <= seen || atomic.CompareAndSwapInt32(&maxConcurrent, seen, cur) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&counter, -1)
				return nil
			})
			if err != nil {
				t.Errorf("withExclusiveLock: %v", err)
			}
		})
	}
	wg.Wait()

	if got := atomic.LoadInt32(&maxConcurrent); got != 1 {
		t.Errorf("exclusive locks must serialize: max concurrent = %d, want 1", got)
	}
}

func TestFileLock_ReleasesOnError(t *testing.T) {
	t.Parallel()
	path := newLockTarget(t)

	sentinel := errors.New("fn failed")
	if err := withExclusiveLock(path, func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("withExclusiveLock should return fn error: got %v", err)
	}
	// A second acquisition would block forever if the lock leaked.
	done := make(chan struct{})
	go func() {
		_ = withExclusiveLock(path, func() error { return nil })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second lock acquisition blocked — lock was not released on error")
	}
}
