//go:build !windows

package main

import "os/exec"

// demoShells returns the command line run through the platform shell.
func demoShells(command string) []*exec.Cmd {
	return []*exec.Cmd{exec.Command("sh", "-c", command)}
}
