package dispatch_test

import (
	"errors"
	"testing"

	"meryl.moe/internal/platform/dispatch"
)

// mockTriggerer records calls made through it and returns a configured error.
// It satisfies the unexported triggerer interface in the dispatch package.
type mockTriggerer struct {
	lastJobName string
	lastPayload any
	err         error
}

func (mock *mockTriggerer) TriggerJob(jobName string, payload any) error {
	mock.lastJobName = jobName
	mock.lastPayload = payload

	return mock.err
}

func TestDispatch_ForwardsJobNameAndPayload(t *testing.T) {
	triggerer := &mockTriggerer{}
	dispatcher := dispatch.New(triggerer)

	if err := dispatcher.Dispatch("test:job", "mydata"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if triggerer.lastJobName != "test:job" {
		t.Errorf("job name: got %q, want %q", triggerer.lastJobName, "test:job")
	}

	if triggerer.lastPayload != "mydata" {
		t.Errorf("payload: got %v, want %q", triggerer.lastPayload, "mydata")
	}
}

func TestDispatch_PropagatesError(t *testing.T) {
	triggerer := &mockTriggerer{err: errors.New("unknown job")}
	dispatcher := dispatch.New(triggerer)

	if err := dispatcher.Dispatch("test:unknown", nil); err == nil {
		t.Error("expected error from triggerer, got nil")
	}
}

func TestNewNoop_DiscardsSilently(t *testing.T) {
	dispatcher := dispatch.NewNoop()

	if err := dispatcher.Dispatch("any:job", "any payload"); err != nil {
		t.Errorf("NewNoop Dispatch returned unexpected error: %v", err)
	}
}
