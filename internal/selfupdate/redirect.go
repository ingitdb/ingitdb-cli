package selfupdate

// specscore: feature/cli/self-update

// UpgradeCommand returns the exact upgrade command a user should run for a
// package-manager-managed install, and true when m is a recognized manager.
// For ManagerNone or any unknown manager it returns ("", false).
//
// ingitdb's managed distribution channels are the Homebrew cask and Snap.
func UpgradeCommand(m Manager) (string, bool) {
	switch m {
	case Homebrew:
		return "brew upgrade --cask ingitdb", true
	case Snap:
		return "snap refresh ingitdb", true
	default:
		return "", false
	}
}

// ManagerName returns a human-readable name for a manager, for display in the
// redirect message.
func ManagerName(m Manager) string {
	switch m {
	case Homebrew:
		return "Homebrew"
	case Snap:
		return "Snap"
	default:
		return "unknown"
	}
}
