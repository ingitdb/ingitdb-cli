//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// openPTY opens a pseudo-terminal pair. It returns the controller side, read
// by the test, and the terminal side, given to the command as stdin and
// stdout, which then sees a real terminal.
func openPTY() (controller, terminal *os.File, err error) {
	controller, err = os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	fd := int(controller.Fd())
	if err = unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		_ = controller.Close()
		return nil, nil, err
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		_ = controller.Close()
		return nil, nil, err
	}
	terminal, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = controller.Close()
		return nil, nil, err
	}
	if _, err = unix.IoctlGetTermios(int(terminal.Fd()), unix.TCGETS); err != nil {
		_ = controller.Close()
		_ = terminal.Close()
		return nil, nil, fmt.Errorf("pty is not a terminal: %w", err)
	}
	return controller, terminal, nil
}
