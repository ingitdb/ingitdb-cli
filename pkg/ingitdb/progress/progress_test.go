package progress_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/progress"
)

// ---------------------------------------------------------------------------
// Compile-time interface satisfaction assertions.
// ---------------------------------------------------------------------------

var _ progress.ProgressReporter = (*mockReporter)(nil)
var _ progress.Steerer = (*mockSteerer)(nil)
var _ progress.Task = (*mockTask)(nil)
var _ progress.Dispatcher = (*mockDispatcher)(nil)

// ---------------------------------------------------------------------------
// Mock implementations.
// ---------------------------------------------------------------------------

type mockReporter struct {
	events []ingitdb.ProgressEvent
}

func (m *mockReporter) Report(event ingitdb.ProgressEvent) {
	m.events = append(m.events, event)
}

type mockSteerer struct {
	next progress.Signal
}

// Steer returns the configured signal then resets to SignalNone, matching
// the interface contract ("resets the signal to SignalNone on read").
func (m *mockSteerer) Steer() progress.Signal {
	s := m.next
	m.next = progress.SignalNone
	return s
}

type mockTask struct {
	taskName string
	runErr   error
}

func (m *mockTask) Name() string { return m.taskName }

func (m *mockTask) Run(
	_ context.Context,
	reporter progress.ProgressReporter,
	_ progress.Steerer,
) error {
	reporter.Report(ingitdb.ProgressEvent{
		Kind:     ingitdb.ProgressKindStarted,
		TaskName: m.taskName,
	})
	return m.runErr
}

type mockDispatcher struct {
	dispErr error
}

func (m *mockDispatcher) RunSequential(ctx context.Context, tasks []progress.Task) error {
	reporter := &mockReporter{}
	steerer := &mockSteerer{}
	for _, task := range tasks {
		if err := task.Run(ctx, reporter, steerer); err != nil {
			return err
		}
	}
	return m.dispErr
}

func (m *mockDispatcher) RunParallel(
	_ context.Context,
	_ []progress.Task,
	_ int,
) error {
	return m.dispErr
}

// ---------------------------------------------------------------------------
// Signal constant tests.
// ---------------------------------------------------------------------------

func TestSignalConstants_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		signal progress.Signal
		want   int
	}{
		{name: "SignalNone is zero", signal: progress.SignalNone, want: 0},
		{name: "SignalSkipItem is one", signal: progress.SignalSkipItem, want: 1},
		{name: "SignalAbort is two", signal: progress.SignalAbort, want: 2},
		{name: "SignalDrillDown is three", signal: progress.SignalDrillDown, want: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if int(tc.signal) != tc.want {
				t.Errorf("signal value = %d, want %d", int(tc.signal), tc.want)
			}
		})
	}
}

func TestSignalConstants_Distinct(t *testing.T) {
	t.Parallel()

	all := []struct {
		name string
		s    progress.Signal
	}{
		{"SignalNone", progress.SignalNone},
		{"SignalSkipItem", progress.SignalSkipItem},
		{"SignalAbort", progress.SignalAbort},
		{"SignalDrillDown", progress.SignalDrillDown},
	}

	seen := make(map[progress.Signal]string, len(all))
	for _, tc := range all {
		if prev, dup := seen[tc.s]; dup {
			t.Errorf("%s and %s share the same Signal value %d", tc.name, prev, tc.s)
		}
		seen[tc.s] = tc.name
	}
}

// ---------------------------------------------------------------------------
// ProgressReporter tests.
// ---------------------------------------------------------------------------

func TestProgressReporter_Report_SingleEvent(t *testing.T) {
	t.Parallel()

	reporter := &mockReporter{}
	event := ingitdb.ProgressEvent{
		Kind:     ingitdb.ProgressKindItemDone,
		TaskName: "validate",
		Scope:    "users",
		ItemKey:  "alice",
		Done:     1,
		Total:    10,
	}

	reporter.Report(event)

	if len(reporter.events) != 1 {
		t.Fatalf("after one Report() len(events) = %d, want 1", len(reporter.events))
	}
	if reporter.events[0] != event {
		t.Errorf("stored event = %+v, want %+v", reporter.events[0], event)
	}
}

func TestProgressReporter_Report_MultipleEvents(t *testing.T) {
	t.Parallel()

	reporter := &mockReporter{}
	kinds := []ingitdb.ProgressKind{
		ingitdb.ProgressKindStarted,
		ingitdb.ProgressKindItemDone,
		ingitdb.ProgressKindCompleted,
	}

	for _, k := range kinds {
		reporter.Report(ingitdb.ProgressEvent{Kind: k, TaskName: "materialize"})
	}

	if len(reporter.events) != len(kinds) {
		t.Fatalf("len(events) = %d, want %d", len(reporter.events), len(kinds))
	}
	for i, k := range kinds {
		if reporter.events[i].Kind != k {
			t.Errorf("events[%d].Kind = %q, want %q", i, reporter.events[i].Kind, k)
		}
	}
}

// ---------------------------------------------------------------------------
// Steerer tests.
// ---------------------------------------------------------------------------

func TestSteerer_Steer_ResetsToNone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured progress.Signal
	}{
		{name: "reset after SkipItem", configured: progress.SignalSkipItem},
		{name: "reset after Abort", configured: progress.SignalAbort},
		{name: "reset after DrillDown", configured: progress.SignalDrillDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := &mockSteerer{next: tc.configured}

			first := s.Steer()
			if first != tc.configured {
				t.Errorf("first Steer() = %v, want %v", first, tc.configured)
			}

			second := s.Steer()
			if second != progress.SignalNone {
				t.Errorf("second Steer() = %v, want SignalNone after reset", second)
			}
		})
	}
}

func TestSteerer_Steer_NoneByDefault(t *testing.T) {
	t.Parallel()

	s := &mockSteerer{} // zero value: next == SignalNone

	got := s.Steer()
	if got != progress.SignalNone {
		t.Errorf("Steer() = %v, want SignalNone", got)
	}
}

// ---------------------------------------------------------------------------
// Task tests.
// ---------------------------------------------------------------------------

func TestTask_Name(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		taskName string
	}{
		{name: "non-empty name", taskName: "materialize"},
		{name: "empty name", taskName: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var task progress.Task = &mockTask{taskName: tc.taskName}
			if got := task.Name(); got != tc.taskName {
				t.Errorf("Name() = %q, want %q", got, tc.taskName)
			}
		})
	}
}

func TestTask_Run_Success(t *testing.T) {
	t.Parallel()

	reporter := &mockReporter{}
	steerer := &mockSteerer{}
	task := &mockTask{taskName: "validate"}

	err := task.Run(context.Background(), reporter, steerer)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(reporter.events) == 0 {
		t.Error("Run() produced no progress events")
	}
	if reporter.events[0].TaskName != "validate" {
		t.Errorf("event TaskName = %q, want %q", reporter.events[0].TaskName, "validate")
	}
}

func TestTask_Run_ReturnsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("task failed")
	reporter := &mockReporter{}
	steerer := &mockSteerer{}
	task := &mockTask{taskName: "import", runErr: wantErr}

	err := task.Run(context.Background(), reporter, steerer)
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if err != wantErr {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestTask_Run_WithCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reporter := &mockReporter{}
	steerer := &mockSteerer{}
	task := &mockTask{taskName: "validate"}

	// The mock does not inspect ctx, but a real Task must. We verify the
	// interface is callable with a cancelled context and the mock behaves.
	err := task.Run(ctx, reporter, steerer)
	if err != nil {
		t.Fatalf("mock Run() with cancelled ctx: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Dispatcher tests.
// ---------------------------------------------------------------------------

func TestDispatcher_RunSequential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tasks   []*mockTask
		dispErr error
		wantErr bool
	}{
		{
			name:  "no tasks",
			tasks: nil,
		},
		{
			name:  "single successful task",
			tasks: []*mockTask{{taskName: "validate"}},
		},
		{
			name:  "multiple successful tasks",
			tasks: []*mockTask{{taskName: "step1"}, {taskName: "step2"}},
		},
		{
			name:    "first task errors stops execution",
			tasks:   []*mockTask{{taskName: "fail", runErr: errors.New("boom")}},
			wantErr: true,
		},
		{
			name:    "dispatcher-level error",
			tasks:   []*mockTask{{taskName: "ok"}},
			dispErr: errors.New("dispatcher failed"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tasks := make([]progress.Task, len(tc.tasks))
			for i, task := range tc.tasks {
				tasks[i] = task
			}

			var d progress.Dispatcher = &mockDispatcher{dispErr: tc.dispErr}
			err := d.RunSequential(context.Background(), tasks)

			if tc.wantErr {
				if err == nil {
					t.Fatal("RunSequential() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("RunSequential() unexpected error: %v", err)
			}
		})
	}
}

func TestDispatcher_RunParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		concurrency int
		dispErr     error
		wantErr     bool
	}{
		{name: "zero concurrency (uses NumCPU)", concurrency: 0},
		{name: "explicit concurrency of 1", concurrency: 1},
		{name: "explicit concurrency of 8", concurrency: 8},
		{
			name:    "error propagation",
			dispErr: errors.New("parallel error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var d progress.Dispatcher = &mockDispatcher{dispErr: tc.dispErr}
			err := d.RunParallel(context.Background(), nil, tc.concurrency)

			if tc.wantErr {
				if err == nil {
					t.Fatal("RunParallel() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("RunParallel() unexpected error: %v", err)
			}
		})
	}
}
