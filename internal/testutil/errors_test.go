package testutil_test

import (
	"errors"
	"testing"

	"github.com/ingitdb/ingitdb-cli/internal/testutil"
)

func TestMustErrContain_Success(t *testing.T) {
	t.Parallel()
	err := errors.New(`frontmatter key "$content" collides with the content field`)
	// Should not call Fatal — both substrings are present.
	testutil.MustErrContain(t, err, "$content", "collide")
}

func TestMustErrContain_FailsOnNilErr(t *testing.T) {
	t.Parallel()
	fake := newFakeTB()
	runWithRecover(func() {
		testutil.MustErrContain(fake, nil, "anything")
	})
	if !fake.failed {
		t.Fatal("expected MustErrContain to fail on nil error")
	}
}

func TestMustErrContain_FailsOnMissingSubstring(t *testing.T) {
	t.Parallel()
	fake := newFakeTB()
	runWithRecover(func() {
		testutil.MustErrContain(fake, errors.New("some error"), "missing")
	})
	if !fake.failed {
		t.Fatal("expected MustErrContain to fail when substring is absent")
	}
}

// runWithRecover invokes fn and swallows any panic. Used to contain the panic
// our fakeTB raises when Fatal/Fatalf is called, so the surrounding test can
// inspect the fake's state afterward.
func runWithRecover(fn func()) {
	defer func() { _ = recover() }()
	fn()
}

// fakeTB embeds *testing.T to satisfy the testing.TB interface (which has
// unexported methods that can only be implemented via embedding). It overrides
// Fatal/Fatalf to record failure and panic, instead of marking the real test
// failed.
type fakeTB struct {
	*testing.T
	failed bool
}

func newFakeTB() *fakeTB {
	return &fakeTB{T: &testing.T{}}
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatal(_ ...any) {
	f.failed = true
	panic("fatal")
}

func (f *fakeTB) Fatalf(_ string, _ ...any) {
	f.failed = true
	panic("fatalf")
}
