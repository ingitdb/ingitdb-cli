package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strongo/cli-helpers/skillsync"
	skillscmd "github.com/strongo/cli-helpers/skillsync/cobracmd"
)

func TestSkillsCmd_Registration(t *testing.T) {
	t.Parallel()
	cmd := Skills("1.0.0")
	if cmd.Name() != "skills" {
		t.Fatalf("expected command name %q, got %q", "skills", cmd.Name())
	}
	if cmd.Short != "Install inGitDB's Agent Skills into a harness's skills directory" {
		t.Fatalf("unexpected short description: %q", cmd.Short)
	}

	syncCmd, _, err := cmd.Find([]string{"sync"})
	if err != nil || syncCmd == nil || syncCmd.Name() != "sync" {
		t.Fatalf("expected 'sync' subcommand on skills, got: %v", syncCmd)
	}

	for _, flag := range []string{"harness", "dir", "dry-run", "newer-compatible", "format"} {
		if syncCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s on 'skills sync' command", flag)
		}
	}
}

func TestNewSkillsConfig(t *testing.T) {
	t.Parallel()
	cfg, err := NewSkillsConfig("1.0.0")
	if err != nil {
		t.Fatalf("NewSkillsConfig error: %v", err)
	}
	if cfg.CLI.Publisher != "ingitdb" || cfg.CLI.Name != "ingitdb" {
		t.Errorf("unexpected CLI identity: %+v", cfg.CLI)
	}
	if len(cfg.Bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(cfg.Bundles))
	}
	bundle := cfg.Bundles[0]
	if bundle.Plugin.Publisher != "ingitdb" || bundle.Plugin.Name != "ingitdb" {
		t.Errorf("unexpected Plugin identity: %+v", bundle.Plugin)
	}
	if bundle.Source.Repository != "ingitdb/ingitdb-cli" {
		t.Errorf("unexpected repository: %s", bundle.Source.Repository)
	}
	if bundle.Source.Digest == "" {
		t.Error("bundle digest is empty")
	}
}

func TestSkillsSyncErrors_Failure(t *testing.T) {
	t.Parallel()
	mapper := skillsSyncErrors{}

	t.Run("usage_error", func(t *testing.T) {
		err := mapper.Failure(&skillscmd.UsageError{Err: errors.New("bad flag")})
		if err == nil || !strings.Contains(err.Error(), "skills: bad flag") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("general_error", func(t *testing.T) {
		err := mapper.Failure(errors.New("sync failed"))
		if err == nil || !strings.Contains(err.Error(), "skills: sync failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSkillsSyncErrors_Conflict(t *testing.T) {
	t.Parallel()
	mapper := skillsSyncErrors{}
	report := skillsync.Report{
		Dir: "/tmp/skills",
	}
	err := mapper.Conflict(report)
	if err == nil || !strings.Contains(err.Error(), "skills sync: 0 skill(s) could not be installed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillsSync_DryRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "skills")

	cmd := Skills("1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"sync", "--dir", targetDir, "--dry-run"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("skills sync --dry-run failed: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, targetDir) {
		t.Errorf("output should mention target dir %s: %s", targetDir, out)
	}

	// In dry-run, directory should not have written actual skill files
	entries, _ := os.ReadDir(targetDir)
	if len(entries) > 0 {
		t.Errorf("expected target dir to have no files written in dry run, got %d entries", len(entries))
	}
}

func TestSkillsSync_RealInstall(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "skills")

	cmd := Skills("1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"sync", "--dir", targetDir})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("skills sync failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify that skills were installed to targetDir
	for _, skillName := range []string{"describe", "install", "list", "record", "validate"} {
		skillFile := filepath.Join(targetDir, skillName, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			t.Errorf("expected installed skill file %s to exist: %v", skillFile, err)
		}
	}
}
