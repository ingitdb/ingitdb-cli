//go:build windows

package main

import (
	"encoding/base64"
	"encoding/binary"
	"os/exec"
	"syscall"
	"unicode/utf16"
)

// demoShells returns the command line run through both Windows shells: cmd.exe
// (given the raw command line, so Go does not re-escape its double quotes) and
// PowerShell (given the command encoded, so its own quote stripping does not
// apply).
func demoShells(command string) []*exec.Cmd {
	cmdExe := exec.Command("cmd.exe")
	cmdExe.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c "` + command + `"`}
	script := utf16.Encode([]rune(command + "; exit $LASTEXITCODE"))
	raw := make([]byte, 2*len(script))
	for i, u := range script {
		binary.LittleEndian.PutUint16(raw[2*i:], u)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	powerShell := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	return []*exec.Cmd{cmdExe, powerShell}
}
