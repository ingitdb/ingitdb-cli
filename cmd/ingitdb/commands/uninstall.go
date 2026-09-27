package commands

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/strongo/cli-helpers/cliinstall/cobracmd"
	"github.com/strongo/cli-helpers/selfupdate"
)

// NewUninstallCommand returns the "uninstall" command built from cliinstall/cobracmd.
func NewUninstallCommand() *cobra.Command {
	return cobracmd.NewUninstall(cobracmd.UninstallCommandOptions{
		Short:  "Uninstall installed fleet CLIs",
		Errors: fleetErrors{prefix: "uninstall"},
		HostID: "ingitdb",
	})
}

// Uninstall returns the "uninstall" command built from cliinstall/cobracmd.
func Uninstall() *cobra.Command {
	return NewUninstallCommand()
}

// fleetErrors maps every cliinstall failure onto ingitdb's error convention.
type fleetErrors struct {
	prefix string
}

func (f fleetErrors) Failure(err error) error {
	prefix := f.prefix
	if prefix == "" {
		prefix = "uninstall"
	}
	var usage *cobracmd.UsageError
	if errors.As(err, &usage) {
		return errors.New(prefix + ": " + err.Error())
	}
	switch selfupdate.KindOf(err) {
	case selfupdate.KindUnknownTarget:
		return errors.New(prefix + ": " + err.Error())
	case selfupdate.KindNoInstallDir, selfupdate.KindDestinationExists:
		return errors.New(prefix + ": " + err.Error())
	default:
		return errors.New(prefix + ": " + err.Error())
	}
}
