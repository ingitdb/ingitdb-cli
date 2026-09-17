//go:build !windows

package main

import "os/exec"

// demoShells returns the command line run through the platform shell; the
// shell name is only used on Windows.
func demoShells(command, _ string) []*exec.Cmd {
	return []*exec.Cmd{exec.Command("sh", "-c", command)}
}
