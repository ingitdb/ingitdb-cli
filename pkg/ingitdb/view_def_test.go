package ingitdb

import (
	"strings"
	"testing"
)

func TestViewDefValidate_MissingID(t *testing.T) {
	t.Parallel()

	v := &ViewDef{}
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "missing 'id' in view definition") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestViewDefValidate_Success(t *testing.T) {
	t.Parallel()

	v := &ViewDef{
		ID:      "readme",
		OrderBy: "title",
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
