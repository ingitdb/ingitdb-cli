package watcher_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/watcher"
)

// ---------------------------------------------------------------------------
// Compile-time interface satisfaction assertions.
// ---------------------------------------------------------------------------

var _ watcher.Watcher = (*mockWatcher)(nil)
var _ watcher.Trigger = (*mockTrigger)(nil)

// ---------------------------------------------------------------------------
// Mock implementations.
// ---------------------------------------------------------------------------

type mockWatcher struct {
	events []watcher.RecordEvent
	err    error
}

func (m *mockWatcher) Watch(ctx context.Context, handler watcher.EventHandler) error {
	for _, e := range m.events {
		handler(e)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return m.err
}

type mockTrigger struct {
	name  string
	err   error
	fired []watcher.RecordEvent
}

func (m *mockTrigger) Name() string { return m.name }

func (m *mockTrigger) Fire(_ context.Context, event watcher.RecordEvent) error {
	m.fired = append(m.fired, event)
	return m.err
}

// ---------------------------------------------------------------------------
// EventType constant tests.
// ---------------------------------------------------------------------------

func TestEventTypeConstants_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType watcher.EventType
		want      string
	}{
		{name: "EventTypeCreated", eventType: watcher.EventTypeCreated, want: "created"},
		{name: "EventTypeModified", eventType: watcher.EventTypeModified, want: "modified"},
		{name: "EventTypeDeleted", eventType: watcher.EventTypeDeleted, want: "deleted"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if string(tc.eventType) != tc.want {
				t.Errorf("EventType value = %q, want %q", tc.eventType, tc.want)
			}
		})
	}
}

func TestEventTypeConstants_Distinct(t *testing.T) {
	t.Parallel()

	all := []struct {
		name string
		et   watcher.EventType
	}{
		{"EventTypeCreated", watcher.EventTypeCreated},
		{"EventTypeModified", watcher.EventTypeModified},
		{"EventTypeDeleted", watcher.EventTypeDeleted},
	}

	seen := make(map[watcher.EventType]string, len(all))
	for _, tc := range all {
		if prev, dup := seen[tc.et]; dup {
			t.Errorf("%s and %s share the same EventType value %q", tc.name, prev, tc.et)
		}
		seen[tc.et] = tc.name
	}
}

// ---------------------------------------------------------------------------
// FieldChange struct tests.
// ---------------------------------------------------------------------------

func TestFieldChange_Construction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fc        watcher.FieldChange
		wantField string
		wantOld   any
		wantNew   any
	}{
		{
			name:      "string values",
			fc:        watcher.FieldChange{Field: "username", OldValue: "alice", NewValue: "alice2"},
			wantField: "username",
			wantOld:   "alice",
			wantNew:   "alice2",
		},
		{
			name:      "integer values",
			fc:        watcher.FieldChange{Field: "score", OldValue: 10, NewValue: 20},
			wantField: "score",
			wantOld:   10,
			wantNew:   20,
		},
		{
			name:      "nil values represent field removal",
			fc:        watcher.FieldChange{Field: "optional"},
			wantField: "optional",
			wantOld:   nil,
			wantNew:   nil,
		},
		{
			name:      "boolean values",
			fc:        watcher.FieldChange{Field: "active", OldValue: false, NewValue: true},
			wantField: "active",
			wantOld:   false,
			wantNew:   true,
		},
		{
			name: "zero value is valid",
			fc:   watcher.FieldChange{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.fc.Field != tc.wantField {
				t.Errorf("Field = %q, want %q", tc.fc.Field, tc.wantField)
			}
			if tc.fc.OldValue != tc.wantOld {
				t.Errorf("OldValue = %v, want %v", tc.fc.OldValue, tc.wantOld)
			}
			if tc.fc.NewValue != tc.wantNew {
				t.Errorf("NewValue = %v, want %v", tc.fc.NewValue, tc.wantNew)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RecordEvent struct tests.
// ---------------------------------------------------------------------------

func TestRecordEvent_Construction(t *testing.T) {
	t.Parallel()

	singleChange := []watcher.FieldChange{
		{Field: "name", OldValue: "bob", NewValue: "robert"},
	}

	tests := []struct {
		name           string
		event          watcher.RecordEvent
		wantType       watcher.EventType
		wantPath       string
		wantChangesLen int
	}{
		{
			name:     "created event with no field changes",
			event:    watcher.RecordEvent{Type: watcher.EventTypeCreated, Path: "users/alice.yaml"},
			wantType: watcher.EventTypeCreated,
			wantPath: "users/alice.yaml",
		},
		{
			name: "modified event with one field change",
			event: watcher.RecordEvent{
				Type:    watcher.EventTypeModified,
				Path:    "users/bob.yaml",
				Changes: singleChange,
			},
			wantType:       watcher.EventTypeModified,
			wantPath:       "users/bob.yaml",
			wantChangesLen: 1,
		},
		{
			name: "modified event with multiple field changes",
			event: watcher.RecordEvent{
				Type: watcher.EventTypeModified,
				Path: "products/item.yaml",
				Changes: []watcher.FieldChange{
					{Field: "price", OldValue: 9.99, NewValue: 12.99},
					{Field: "stock", OldValue: 100, NewValue: 95},
				},
			},
			wantType:       watcher.EventTypeModified,
			wantPath:       "products/item.yaml",
			wantChangesLen: 2,
		},
		{
			name:     "deleted event with no changes",
			event:    watcher.RecordEvent{Type: watcher.EventTypeDeleted, Path: "old/record.yaml"},
			wantType: watcher.EventTypeDeleted,
			wantPath: "old/record.yaml",
		},
		{
			name:  "zero value is valid",
			event: watcher.RecordEvent{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.event.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", tc.event.Type, tc.wantType)
			}
			if tc.event.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", tc.event.Path, tc.wantPath)
			}
			if len(tc.event.Changes) != tc.wantChangesLen {
				t.Errorf("len(Changes) = %d, want %d", len(tc.event.Changes), tc.wantChangesLen)
			}
		})
	}
}

func TestRecordEvent_FieldChanges_Content(t *testing.T) {
	t.Parallel()

	event := watcher.RecordEvent{
		Type: watcher.EventTypeModified,
		Path: "items/widget.yaml",
		Changes: []watcher.FieldChange{
			{Field: "color", OldValue: "red", NewValue: "blue"},
			{Field: "weight", OldValue: 1.5, NewValue: 2.0},
		},
	}

	if event.Changes[0].Field != "color" {
		t.Errorf("Changes[0].Field = %q, want %q", event.Changes[0].Field, "color")
	}
	if event.Changes[0].OldValue != "red" {
		t.Errorf("Changes[0].OldValue = %v, want %q", event.Changes[0].OldValue, "red")
	}
	if event.Changes[0].NewValue != "blue" {
		t.Errorf("Changes[0].NewValue = %v, want %q", event.Changes[0].NewValue, "blue")
	}
	if event.Changes[1].Field != "weight" {
		t.Errorf("Changes[1].Field = %q, want %q", event.Changes[1].Field, "weight")
	}
}

// ---------------------------------------------------------------------------
// EventHandler func-type tests.
// ---------------------------------------------------------------------------

func TestEventHandler_Callable(t *testing.T) {
	t.Parallel()

	called := false
	var captured watcher.RecordEvent

	var h watcher.EventHandler = func(event watcher.RecordEvent) {
		called = true
		captured = event
	}

	expected := watcher.RecordEvent{Type: watcher.EventTypeCreated, Path: "items/new.yaml"}
	h(expected)

	if !called {
		t.Fatal("EventHandler was not called")
	}
	if !reflect.DeepEqual(captured, expected) {
		t.Errorf("EventHandler received %+v, want %+v", captured, expected)
	}
}

func TestEventHandler_CalledMultipleTimes(t *testing.T) {
	t.Parallel()

	events := []watcher.RecordEvent{
		{Type: watcher.EventTypeCreated, Path: "a.yaml"},
		{Type: watcher.EventTypeModified, Path: "b.yaml"},
		{Type: watcher.EventTypeDeleted, Path: "c.yaml"},
	}

	var received []watcher.RecordEvent
	var h watcher.EventHandler = func(event watcher.RecordEvent) {
		received = append(received, event)
	}

	for _, e := range events {
		h(e)
	}

	if len(received) != len(events) {
		t.Fatalf("handler called %d times, want %d", len(received), len(events))
	}
	for i, e := range events {
		if !reflect.DeepEqual(received[i], e) {
			t.Errorf("received[%d] = %+v, want %+v", i, received[i], e)
		}
	}
}

// ---------------------------------------------------------------------------
// Watcher interface tests (via mock).
// ---------------------------------------------------------------------------

func TestWatcher_Watch_DeliverEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		events    []watcher.RecordEvent
		wantCount int
		wantFirst watcher.RecordEvent
	}{
		{
			name:      "no events",
			events:    nil,
			wantCount: 0,
		},
		{
			name:      "empty slice",
			events:    []watcher.RecordEvent{},
			wantCount: 0,
		},
		{
			name: "single created event",
			events: []watcher.RecordEvent{
				{Type: watcher.EventTypeCreated, Path: "users/alice.yaml"},
			},
			wantCount: 1,
			wantFirst: watcher.RecordEvent{Type: watcher.EventTypeCreated, Path: "users/alice.yaml"},
		},
		{
			name: "multiple events of mixed types",
			events: []watcher.RecordEvent{
				{Type: watcher.EventTypeCreated, Path: "a.yaml"},
				{Type: watcher.EventTypeModified, Path: "b.yaml"},
				{Type: watcher.EventTypeDeleted, Path: "c.yaml"},
			},
			wantCount: 3,
			wantFirst: watcher.RecordEvent{Type: watcher.EventTypeCreated, Path: "a.yaml"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var received []watcher.RecordEvent
			var w watcher.Watcher = &mockWatcher{events: tc.events}

			err := w.Watch(context.Background(), func(e watcher.RecordEvent) {
				received = append(received, e)
			})
			if err != nil {
				t.Fatalf("Watch() unexpected error: %v", err)
			}
			if len(received) != tc.wantCount {
				t.Fatalf("Watch() delivered %d events, want %d", len(received), tc.wantCount)
			}
			if tc.wantCount > 0 && !reflect.DeepEqual(received[0], tc.wantFirst) {
				t.Errorf("Watch() first event = %+v, want %+v", received[0], tc.wantFirst)
			}
		})
	}
}

func TestWatcher_Watch_ErrorPropagation(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("watch error")
	var w watcher.Watcher = &mockWatcher{err: wantErr}

	err := w.Watch(context.Background(), func(watcher.RecordEvent) {})
	if err == nil {
		t.Fatal("Watch() expected error, got nil")
	}
	if err != wantErr {
		t.Errorf("Watch() error = %v, want %v", err, wantErr)
	}
}

func TestWatcher_Watch_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	events := []watcher.RecordEvent{
		{Type: watcher.EventTypeCreated, Path: "x.yaml"},
		{Type: watcher.EventTypeCreated, Path: "y.yaml"},
	}

	var callCount int
	var w watcher.Watcher = &mockWatcher{events: events}

	err := w.Watch(ctx, func(watcher.RecordEvent) { callCount++ })
	if err == nil {
		t.Fatal("Watch() with cancelled context should return an error")
	}
	// The mock calls the handler once before detecting cancellation.
	if callCount > 1 {
		t.Errorf("Watch() invoked handler %d times with pre-cancelled context, want ≤ 1", callCount)
	}
}

// ---------------------------------------------------------------------------
// Trigger interface tests (via mock).
// ---------------------------------------------------------------------------

func TestTrigger_Name(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		triggerName string
	}{
		{name: "named trigger", triggerName: "webhook"},
		{name: "another name", triggerName: "shell-exec"},
		{name: "empty name", triggerName: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var tr watcher.Trigger = &mockTrigger{name: tc.triggerName}
			if got := tr.Name(); got != tc.triggerName {
				t.Errorf("Name() = %q, want %q", got, tc.triggerName)
			}
		})
	}
}

func TestTrigger_Fire_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		events []watcher.RecordEvent
	}{
		{
			name:   "no events",
			events: nil,
		},
		{
			name: "single event",
			events: []watcher.RecordEvent{
				{Type: watcher.EventTypeCreated, Path: "new.yaml"},
			},
		},
		{
			name: "multiple events",
			events: []watcher.RecordEvent{
				{Type: watcher.EventTypeCreated, Path: "a.yaml"},
				{Type: watcher.EventTypeModified, Path: "b.yaml"},
				{Type: watcher.EventTypeDeleted, Path: "c.yaml"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := &mockTrigger{name: "test-trigger"}
			var iface watcher.Trigger = tr

			for _, event := range tc.events {
				err := iface.Fire(context.Background(), event)
				if err != nil {
					t.Fatalf("Fire() unexpected error: %v", err)
				}
			}

			if len(tr.fired) != len(tc.events) {
				t.Errorf("fired count = %d, want %d", len(tr.fired), len(tc.events))
			}
			for i, e := range tc.events {
				if !reflect.DeepEqual(tr.fired[i], e) {
					t.Errorf("fired[%d] = %+v, want %+v", i, tr.fired[i], e)
				}
			}
		})
	}
}

func TestTrigger_Fire_ErrorPropagation(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("webhook unreachable")
	var tr watcher.Trigger = &mockTrigger{name: "webhook", err: wantErr}

	event := watcher.RecordEvent{Type: watcher.EventTypeModified, Path: "item.yaml"}
	err := tr.Fire(context.Background(), event)

	if err == nil {
		t.Fatal("Fire() expected error, got nil")
	}
	if err != wantErr {
		t.Errorf("Fire() error = %v, want %v", err, wantErr)
	}
}

func TestTrigger_Fire_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The mock ignores ctx, but the interface contract requires implementations
	// to respect it. We verify the interface is callable with a cancelled context.
	var tr watcher.Trigger = &mockTrigger{name: "shell"}
	event := watcher.RecordEvent{Type: watcher.EventTypeCreated, Path: "new.yaml"}

	err := tr.Fire(ctx, event)
	if err != nil {
		t.Fatalf("mock Fire() with cancelled ctx: unexpected error: %v", err)
	}
}
