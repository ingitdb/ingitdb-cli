package commands

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"
	"github.com/strongo/buildinfo"
	"github.com/strongo/cli-helpers/skillsync"
	skillscmd "github.com/strongo/cli-helpers/skillsync/cobracmd"
	"github.com/strongo/cli-helpers/skillsync/githubrelease"

	"github.com/ingitdb/ingitdb-cli/ai"
)

const (
	ingitdbSkillsPluginVersion = "0.0.0"
	ingitdbSkillsUnknownSource = "0000000000000000000000000000000000000000"
)

var (
	ingitdbSkillsCLI    = skillsync.Identity{Publisher: "ingitdb", Name: "ingitdb"}
	ingitdbSkillsPlugin = skillsync.PluginIdentity{Publisher: "ingitdb", Name: "ingitdb"}
)

// Skills returns the "skills" command built from skillsync/cobracmd.
func Skills(currentVersion string) *cobra.Command {
	cfg, cfgErr := NewSkillsConfig(currentVersion)
	options := skillscmd.CommandOptions{
		Short:  "Install inGitDB's Agent Skills into a harness's skills directory",
		Errors: skillsSyncErrors{},
		Resolver: skillsync.ReleaseResolver{
			Source:         githubrelease.Source{},
			CurrentVersion: cfg.CurrentVersion,
		},
	}
	command := skillscmd.New(cfg, options)
	if cfgErr != nil {
		command.RunE = func(*cobra.Command, []string) error {
			return skillsSyncErrors{}.Failure(fmt.Errorf("prepare embedded inGitDB skills: %w", cfgErr))
		}
	}
	return command
}

// NewSkillsConfig builds the skillsync.Config for embedded inGitDB skills.
func NewSkillsConfig(currentVersion string) (skillsync.Config, error) {
	source, err := fs.Sub(ai.SkillsFS, "skills")
	if err != nil {
		return skillsync.Config{}, err
	}
	digest, err := skillsync.Digest(source)
	if err != nil {
		return skillsync.Config{}, err
	}
	info := buildinfo.Get("ingitdb")
	revision := info.Commit
	if len(revision) != 40 {
		revision = ingitdbSkillsUnknownSource
	}
	pluginVersion := currentVersion
	if _, err := skillsync.CompareVersions(pluginVersion, pluginVersion); err != nil {
		pluginVersion = ingitdbSkillsPluginVersion
	}
	bundle, err := skillsync.EmbeddedBundle(skillsync.BundleDescriptor{
		Plugin: ingitdbSkillsPlugin,
		Source: skillsync.Source{
			Repository: "ingitdb/ingitdb-cli",
			Path:       "ai/skills",
			Revision:   revision,
			Version:    pluginVersion,
			Digest:     digest,
		},
	}, source)
	if err != nil {
		return skillsync.Config{}, err
	}
	return skillsync.Config{
		CLI:            ingitdbSkillsCLI,
		CurrentVersion: currentVersion,
		Bundles:        []skillsync.Bundle{bundle},
	}, nil
}

type skillsSyncErrors struct{}

func (skillsSyncErrors) Failure(err error) error {
	var usage *skillscmd.UsageError
	if errors.As(err, &usage) {
		return errors.New("skills: " + err.Error())
	}
	return errors.New("skills: " + err.Error())
}

func (skillsSyncErrors) Conflict(report skillsync.Report) error {
	return fmt.Errorf(
		"skills sync: %d skill(s) could not be installed because another plugin or an unmanaged directory already owns the name; see %s",
		len(report.Names(skillsync.Conflict)), report.Dir)
}
