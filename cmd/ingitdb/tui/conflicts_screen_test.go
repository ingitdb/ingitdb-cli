package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestConflictsModel_InitNil(t *testing.T) {
	t.Parallel()
	m := NewConflictsModel([]string{"a.yaml"}, 80, 24)
	if m.Init() != nil {
		t.Error("expected nil Init command")
	}
}

func TestConflictsModel_Update_QuitKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"q", "esc", "enter"} {
		m := NewConflictsModel([]string{"a.yaml"}, 80, 24)
		_, cmd := m.Update(tea.KeyPressMsg{Text: key})
		if cmd == nil {
			t.Errorf("key %q: expected quit command", key)
		}
	}
	// ctrl+c via modifier.
	m := NewConflictsModel(nil, 80, 24)
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
		t.Error("ctrl+c: expected quit command")
	}
}

func TestConflictsModel_Update_OtherKeyNoQuit(t *testing.T) {
	t.Parallel()
	m := NewConflictsModel([]string{"a.yaml"}, 80, 24)
	if _, cmd := m.Update(tea.KeyPressMsg{Text: "x"}); cmd != nil {
		t.Error("expected no command for unrelated key")
	}
}

func TestConflictsModel_Update_WindowSize(t *testing.T) {
	t.Parallel()
	m := NewConflictsModel(nil, 80, 24)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 150, Height: 40})
	if cmd != nil {
		t.Error("window size should not emit a command")
	}
	cm, ok := updated.(ConflictsModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	if cm.width != 150 || cm.height != 40 {
		t.Errorf("size not applied: %dx%d", cm.width, cm.height)
	}
}

func TestConflictsModel_View(t *testing.T) {
	t.Parallel()
	// Narrow width exercises the panelWidth floor; the View must include the
	// title, the file, and the not-implemented notice.
	m := NewConflictsModel([]string{"data/users/u1.yaml"}, 10, 24)
	out := m.View().Content
	for _, want := range []string{"Interactive conflict resolution", "u1.yaml", "not implemented yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q\n%s", want, out)
		}
	}
	// Wide width exercises the panelWidth ceiling.
	wide := NewConflictsModel([]string{"x.yaml"}, 400, 50)
	if wide.View().Content == "" {
		t.Error("expected non-empty wide view")
	}
}

func TestRunConflicts_CancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // bubbletea exits immediately; no real TTY needed.
	_ = RunConflicts(ctx, []string{"a.yaml"}, 80, 24)
}
