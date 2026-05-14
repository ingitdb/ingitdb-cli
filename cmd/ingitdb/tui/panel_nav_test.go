package tui

import "testing"

// ---------------------------------------------------------------------------
// panelNav.HandleKey
// ---------------------------------------------------------------------------

func TestPanelNav_HandleKey_AltLeft(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 2}
	consumed := p.HandleKey("alt+left")
	if !consumed {
		t.Error("alt+left should be consumed")
	}
	if p.focus != 1 {
		t.Errorf("focus = %d, want 1", p.focus)
	}
}

func TestPanelNav_HandleKey_AltLeft_AtMin(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 0}
	consumed := p.HandleKey("alt+left")
	if !consumed {
		t.Error("alt+left should be consumed even at minimum")
	}
	if p.focus != 0 {
		t.Errorf("focus = %d, want 0 (no change at boundary)", p.focus)
	}
}

func TestPanelNav_HandleKey_AltRight(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 0}
	consumed := p.HandleKey("alt+right")
	if !consumed {
		t.Error("alt+right should be consumed")
	}
	if p.focus != 1 {
		t.Errorf("focus = %d, want 1", p.focus)
	}
}

func TestPanelNav_HandleKey_AltRight_AtMax(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 2}
	consumed := p.HandleKey("alt+right")
	if !consumed {
		t.Error("alt+right should be consumed even at maximum")
	}
	if p.focus != 2 {
		t.Errorf("focus = %d, want 2 (no change at boundary)", p.focus)
	}
}

func TestPanelNav_HandleKey_Other(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 1}
	consumed := p.HandleKey("enter")
	if consumed {
		t.Error("enter should not be consumed by panelNav")
	}
	if p.focus != 1 {
		t.Errorf("focus changed unexpectedly to %d", p.focus)
	}
}

func TestPanelNav_IsFocused(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 1}
	if !p.IsFocused(1) {
		t.Error("IsFocused(1) should be true")
	}
	if p.IsFocused(0) {
		t.Error("IsFocused(0) should be false")
	}
	if p.IsFocused(2) {
		t.Error("IsFocused(2) should be false")
	}
}

func TestPanelNav_Style_FocusedVsUnfocused(t *testing.T) {
	t.Parallel()
	p := panelNav{count: 3, focus: 1}
	// Render a string with each style and verify the output differs.
	// focusedPanelStyle has a purple border, panelStyle has a grey border.
	focusedRender := p.Style(1).Render("x")
	unfocusedRender := p.Style(0).Render("x")
	if focusedRender == unfocusedRender {
		t.Error("focused style render should differ from unfocused style render")
	}
}
