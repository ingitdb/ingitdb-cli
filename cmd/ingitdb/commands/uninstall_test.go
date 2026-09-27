package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/strongo/cli-helpers/cliinstall/cobracmd"
	"github.com/strongo/cli-helpers/selfupdate"
)

func TestUninstallCmd_Registration(t *testing.T) {
	t.Parallel()
	cmd := NewUninstallCommand()
	if cmd.Name() != "uninstall" {
		t.Fatalf("expected command name %q, got %q", "uninstall", cmd.Name())
	}
	if cmd.Short != "Uninstall installed fleet CLIs" {
		t.Fatalf("unexpected short description: %q", cmd.Short)
	}

	flags := []string{"all", "dry-run", "yes", "purge", "format"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag --%s on uninstall command", name)
		}
	}

	aliasCmd := Uninstall()
	if aliasCmd.Name() != "uninstall" {
		t.Fatalf("expected alias command name %q, got %q", "uninstall", aliasCmd.Name())
	}
}

func TestFleetErrors_Failure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mapper fleetErrors
		prefix string
	}{
		{
			name:   "explicit_prefix",
			mapper: fleetErrors{prefix: "uninstall"},
			prefix: "uninstall",
		},
		{
			name:   "default_prefix",
			mapper: fleetErrors{},
			prefix: "uninstall",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mapper := tt.mapper
			prefix := tt.prefix

			t.Run("usage_error", func(t *testing.T) {
				err := mapper.Failure(&cobracmd.UsageError{Err: errors.New("bad flag")})
				if err == nil || !strings.Contains(err.Error(), prefix+": bad flag") {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			t.Run("unknown_target", func(t *testing.T) {
				failure := &selfupdate.Failure{Kind: selfupdate.KindUnknownTarget, Err: errors.New("unknown cli")}
				err := mapper.Failure(failure)
				if err == nil || !strings.Contains(err.Error(), prefix+": unknown cli") {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			t.Run("no_install_dir", func(t *testing.T) {
				failure := &selfupdate.Failure{Kind: selfupdate.KindNoInstallDir, Err: errors.New("no bin dir")}
				err := mapper.Failure(failure)
				if err == nil || !strings.Contains(err.Error(), prefix+": no bin dir") {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			t.Run("destination_exists", func(t *testing.T) {
				failure := &selfupdate.Failure{Kind: selfupdate.KindDestinationExists, Err: errors.New("exists")}
				err := mapper.Failure(failure)
				if err == nil || !strings.Contains(err.Error(), prefix+": exists") {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			t.Run("general_error", func(t *testing.T) {
				err := mapper.Failure(errors.New("something went wrong"))
				if err == nil || !strings.Contains(err.Error(), prefix+": something went wrong") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
	}
}

func TestUninstallCmd_Execution_UnknownTarget(t *testing.T) {
	t.Parallel()
	cmd := NewUninstallCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"nosuchclitarget"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
	if !strings.Contains(err.Error(), "uninstall: ") {
		t.Fatalf("expected error to contain %q, got %q", "uninstall: ", err.Error())
	}
}

func TestUninstallCmd_Execution_NoArgsNoAll(t *testing.T) {
	t.Parallel()
	cmd := NewUninstallCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected usage error when neither target nor --all is specified")
	}
	if !strings.Contains(err.Error(), "uninstall: ") {
		t.Fatalf("expected error to contain %q, got %q", "uninstall: ", err.Error())
	}
}
