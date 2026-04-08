package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegistrar_Register_JobsAvailableAfterStart(t *testing.T) {
	registrar := newRegistrar()

	registrar.Register(Job{
		Name: "test:alpha",
		Run:  func(ctx context.Context, payload any) error { return nil },
	})
	registrar.Register(Job{
		Name: "test:beta",
		Run:  func(ctx context.Context, payload any) error { return nil },
	})

	runner := registrar.JobRunner()
	runner.Start(context.Background())

	if err := runner.TriggerJob("test:alpha", nil); err != nil {
		t.Errorf("test:alpha not found after Start: %v", err)
	}

	if err := runner.TriggerJob("test:beta", nil); err != nil {
		t.Errorf("test:beta not found after Start: %v", err)
	}
}

func TestJobRunner_TriggerJob_ExecutesJob(t *testing.T) {
	registrar := newRegistrar()
	done := make(chan struct{}, 1)

	registrar.Register(Job{
		Name: "test:execute",
		Run: func(ctx context.Context, payload any) error {
			done <- struct{}{}
			return nil
		},
	})

	runner := registrar.JobRunner()
	runner.Start(context.Background())

	if err := runner.TriggerJob("test:execute", nil); err != nil {
		t.Fatalf("TriggerJob: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("job did not execute within timeout")
	}
}

func TestJobRunner_TriggerJob_PassesPayload(t *testing.T) {
	registrar := newRegistrar()
	received := make(chan any, 1)

	registrar.Register(Job{
		Name: "test:payload",
		Run: func(ctx context.Context, payload any) error {
			received <- payload
			return nil
		},
	})

	runner := registrar.JobRunner()
	runner.Start(context.Background())

	if err := runner.TriggerJob("test:payload", "hello"); err != nil {
		t.Fatalf("TriggerJob: %v", err)
	}

	select {
	case payload := <-received:
		if payload != "hello" {
			t.Errorf("payload: got %v, want %q", payload, "hello")
		}
	case <-time.After(time.Second):
		t.Error("job did not execute within timeout")
	}
}

func TestJobRunner_TriggerJob_UnknownJob_ReturnsError(t *testing.T) {
	runner := newRegistrar().JobRunner()
	runner.Start(context.Background())

	if err := runner.TriggerJob("no:such:job", nil); err == nil {
		t.Error("expected error for unknown job name, got nil")
	}
}

func TestJobRunner_TriggerJob_JobError_RunnerContinues(t *testing.T) {
	registrar := newRegistrar()
	done := make(chan struct{}, 1)

	registrar.Register(Job{
		Name: "test:fails",
		Run:  func(ctx context.Context, payload any) error { return errors.New("job failed") },
	})
	registrar.Register(Job{
		Name: "test:succeeds",
		Run:  func(ctx context.Context, payload any) error { done <- struct{}{}; return nil },
	})

	runner := registrar.JobRunner()
	runner.Start(context.Background())

	if err := runner.TriggerJob("test:fails", nil); err != nil {
		t.Errorf("TriggerJob returned error for internally-failing job: %v", err)
	}

	if err := runner.TriggerJob("test:succeeds", nil); err != nil {
		t.Fatalf("TriggerJob: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("runner stopped working after a job error")
	}
}

func TestJobRunner_Start_ScheduledJob_RunsImmediately(t *testing.T) {
	registrar := newRegistrar()
	executed := make(chan struct{}, 1)

	registrar.Register(Job{
		Name:     "test:scheduled",
		Interval: time.Hour, // long interval - only the immediate run matters here
		Run: func(ctx context.Context, payload any) error {
			select {
			case executed <- struct{}{}:
			default:
			}
			return nil
		},
	})

	runner := registrar.JobRunner()
	runner.Start(t.Context())

	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Error("scheduled job did not run immediately on Start")
	}
}

func TestJobRunner_Start_OnDemandJob_DoesNotRunAutomatically(t *testing.T) {
	registrar := newRegistrar()
	executed := make(chan struct{}, 1)

	registrar.Register(Job{
		Name: "test:ondemand",
		// Interval = 0: on-demand only
		Run: func(ctx context.Context, payload any) error {
			executed <- struct{}{}
			return nil
		},
	})

	runner := registrar.JobRunner()
	runner.Start(t.Context())

	select {
	case <-executed:
		t.Error("on-demand job ran automatically after Start")
	case <-time.After(50 * time.Millisecond):
		// correct: on-demand job did not run
	}
}
