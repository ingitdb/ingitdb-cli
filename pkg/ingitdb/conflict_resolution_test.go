package ingitdb

import "testing"

func boolPtr(b bool) *bool { return &b }

func crc(rm *RecordMergeConfig) *ConflictResolutionConfig {
	return &ConflictResolutionConfig{RecordMerge: rm}
}

func TestResolveRecordMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		def            *Definition
		col            *CollectionDef
		wantEnabled    bool
		wantSameRecord bool
	}{
		{
			name:        "defaults when both nil",
			wantEnabled: true,
		},
		{
			name:        "def settings has no conflict-resolution block",
			def:         &Definition{},
			col:         &CollectionDef{},
			wantEnabled: true,
		},
		{
			name:        "conflict-resolution present but record_merge nil",
			def:         &Definition{Settings: Settings{ConflictResolution: crc(nil)}},
			wantEnabled: true,
		},
		{
			name:        "db disables record merge",
			def:         &Definition{Settings: Settings{ConflictResolution: crc(&RecordMergeConfig{Enabled: boolPtr(false)})}},
			wantEnabled: false,
		},
		{
			name:           "db enables same-record",
			def:            &Definition{Settings: Settings{ConflictResolution: crc(&RecordMergeConfig{SameRecord: boolPtr(true)})}},
			wantEnabled:    true,
			wantSameRecord: true,
		},
		{
			name:        "collection override disables what db enabled",
			def:         &Definition{Settings: Settings{ConflictResolution: crc(&RecordMergeConfig{Enabled: boolPtr(true)})}},
			col:         &CollectionDef{ConflictResolution: crc(&RecordMergeConfig{Enabled: boolPtr(false)})},
			wantEnabled: false,
		},
		{
			name:           "collection override enables same-record",
			def:            &Definition{Settings: Settings{ConflictResolution: crc(&RecordMergeConfig{SameRecord: boolPtr(false)})}},
			col:            &CollectionDef{ConflictResolution: crc(&RecordMergeConfig{SameRecord: boolPtr(true)})},
			wantEnabled:    true,
			wantSameRecord: true,
		},
		{
			name:           "collection sets only same-record, inherits db enabled",
			def:            &Definition{Settings: Settings{ConflictResolution: crc(&RecordMergeConfig{Enabled: boolPtr(false)})}},
			col:            &CollectionDef{ConflictResolution: crc(&RecordMergeConfig{SameRecord: boolPtr(true)})},
			wantEnabled:    false,
			wantSameRecord: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveRecordMerge(tt.def, tt.col)
			if got.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.wantEnabled)
			}
			if got.SameRecord != tt.wantSameRecord {
				t.Errorf("SameRecord = %v, want %v", got.SameRecord, tt.wantSameRecord)
			}
		})
	}
}
