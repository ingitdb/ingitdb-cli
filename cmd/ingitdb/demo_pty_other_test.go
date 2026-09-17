//go:build !linux

package main

import (
	"errors"
	"os"
)

// openPTY is only implemented on Linux. macOS needs posix_openpt/grantpt,
// which the Go standard library and golang.org/x/sys do not wrap without cgo,
// and Windows has ConPTY rather than a terminal file; the no-prompt check
// there is the pipe run plus the structural check in the commands package.
func openPTY() (controller, terminal *os.File, err error) {
	return nil, nil, errors.New("pseudo-terminals are only opened on linux")
}
