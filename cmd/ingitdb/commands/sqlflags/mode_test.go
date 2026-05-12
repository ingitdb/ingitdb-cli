package sqlflags

import "testing"

func TestResolveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		from    string
		want    Mode
		wantErr bool
	}{
		{name: "id only", id: "countries/ie", from: "", want: ModeID},
		{name: "from only", id: "", from: "countries", want: ModeFrom},
		{name: "neither rejected", id: "", from: "", wantErr: true},
		{name: "both rejected", id: "countries/ie", from: "countries", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveMode(tt.id, tt.from)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
