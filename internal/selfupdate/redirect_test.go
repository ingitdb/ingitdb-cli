package selfupdate

import "testing"

func TestUpgradeCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		manager Manager
		want    string
		wantOK  bool
	}{
		{"homebrew cask", Homebrew, "brew upgrade --cask ingitdb", true},
		{"snap", Snap, "snap refresh ingitdb", true},
		{"none", ManagerNone, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := UpgradeCommand(c.manager)
			if got != c.want || ok != c.wantOK {
				t.Errorf("UpgradeCommand(%v) = (%q, %v); want (%q, %v)",
					c.manager, got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestManagerName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		m    Manager
		want string
	}{
		{Homebrew, "Homebrew"},
		{Snap, "Snap"},
		{ManagerNone, "unknown"},
	}
	for _, c := range cases {
		if got := ManagerName(c.m); got != c.want {
			t.Errorf("ManagerName(%v) = %q, want %q", c.m, got, c.want)
		}
	}
}
