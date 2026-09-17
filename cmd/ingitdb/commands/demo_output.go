package commands

// specscore: feature/cli/demo

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-go/ingitdb/demos/todo"
)

// demoResult is the result of `ingitdb demo install`, printed as text or as
// the structured document of cli/demo#REQ:structured-output.
type demoResult struct {
	App              string         `json:"app" yaml:"app"`
	Installed        bool           `json:"installed" yaml:"installed"`
	AlreadyInstalled bool           `json:"already_installed" yaml:"already_installed"`
	Path             string         `json:"path" yaml:"path"`
	Git              demoGitResult  `json:"git" yaml:"git"`
	Lists            []string       `json:"lists" yaml:"lists"`
	Next             []demoNextStep `json:"next" yaml:"next"`
}

type demoGitResult struct {
	Repository bool   `json:"repository" yaml:"repository"`
	Commit     string `json:"commit" yaml:"commit"`
}

// demoNextStep is one command of the What next? list. Step is the 1-based
// number of the step the command belongs to; a step with several commands,
// such as the OpenVaultDB one, has one entry per command, in order.
type demoNextStep struct {
	Step    int    `json:"step" yaml:"step"`
	Label   string `json:"label" yaml:"label"`
	Command string `json:"command" yaml:"command"`
	Note    string `json:"note,omitempty" yaml:"note,omitempty"`
}

const (
	demoOVDBLabel = "Use the lists in a web TODO app (OpenVaultDB)"
	demoOVDBNote  = "OpenVaultDB keeps its own copy of the same lists. Install ovdb from https://github.com/openvaultdb/ovdb"
)

func newDemoResult(dir string) demoResult {
	return demoResult{
		App:       demoApp,
		Installed: true,
		Path:      dir,
		Lists:     slices.Clone(todo.Lists),
	}
}

// demoNextSteps returns the next steps in the order of cli/demo#REQ:next-steps,
// with dir quoted for the shell of goos.
func demoNextSteps(dir, goos string) []demoNextStep {
	pathArg := "--path=" + demoShellQuote(dir, goos)
	firstList := todo.Lists[0]
	collection, _, _ := strings.Cut(firstList, "/")
	return []demoNextStep{
		{Step: 1, Label: "Browse the lists in the terminal UI", Command: "ingitdb " + pathArg},
		{Step: 2, Label: "Query the lists", Command: "ingitdb select --from=" + collection + " " + pathArg},
		{Step: 3, Label: "Query what to buy", Command: "ingitdb select --from=" + firstList + "/" + demoItemsCollection + " " + pathArg},
		{Step: 4, Label: demoOVDBLabel, Command: "ovdb demo install --yes"},
		{Step: 4, Label: demoOVDBLabel, Command: "ovdb demo open", Note: demoOVDBNote},
	}
}

// demoShellQuote quotes s for pasting into the shell of goos when it holds
// anything but plain path characters: double quotes on Windows, so the
// command works in both cmd.exe and PowerShell, POSIX single quotes
// elsewhere.
func demoShellQuote(s, goos string) string {
	safe := func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return true
		case strings.ContainsRune("-_./:+", r):
			return true
		case r == '\\':
			return goos == "windows"
		}
		return false
	}
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !safe(r) }) < 0 {
		return s
	}
	if goos == "windows" {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// writeDemoResult prints result in format ("" for text, "yaml" or "json").
func writeDemoResult(w io.Writer, result demoResult, format, goos string) error {
	result.Next = demoNextSteps(result.Path, goos)
	var out []byte
	switch format {
	case "json":
		out, _ = json.MarshalIndent(result, "", "  ")
		out = append(out, '\n')
	case "yaml":
		out, _ = yaml.Marshal(result)
	default:
		out = []byte(demoText(result))
	}
	_, err := w.Write(out)
	return err
}

// demoText renders the human-readable result (cli/demo#REQ:human-output).
func demoText(result demoResult) string {
	var b strings.Builder
	if result.AlreadyInstalled {
		fmt.Fprintf(&b, "The TODO demo is already installed in %s\n\n", result.Path)
	} else {
		b.WriteString("The TODO demo is ready\n\n")
	}
	fmt.Fprintf(&b, "Lists:     %s\n", strings.Join(result.Lists, ", "))
	fmt.Fprintf(&b, "Stored in: %s\n", result.Path)
	switch {
	case !result.Git.Repository:
		b.WriteString("Git:       not a Git repository; run `git init` in the folder to add history\n")
	case result.Git.Commit == "":
		b.WriteString("Git:       a Git repository\n")
	default:
		fmt.Fprintf(&b, "Git:       a Git repository, commit %s\n", result.Git.Commit[:min(7, len(result.Git.Commit))])
	}
	b.WriteString("\nWhat next?\n")
	for idx, step := range result.Next {
		if idx == 0 || result.Next[idx-1].Step != step.Step {
			fmt.Fprintf(&b, "  %d. %s\n", step.Step, step.Label)
		}
		fmt.Fprintf(&b, "       %s\n", step.Command)
		if step.Note != "" {
			fmt.Fprintf(&b, "       %s\n", step.Note)
		}
	}
	return b.String()
}
