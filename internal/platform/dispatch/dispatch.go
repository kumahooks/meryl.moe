// Package dispatch provides a work dispatcher for triggering background jobs
// from within module handlers without direct coupling to the worker package.
package dispatch

// Job name constants exposed to Dispatch
const (
	RelayCleanup = "relay:cleanup"

	// TODO: remove - test job to verify dispatch mechanism
	AuthLogin = "auth:login"
)

type triggerer interface {
	TriggerJob(jobName string, payload any) error
}

type noopTriggerer struct{}

func (noopTriggerer) TriggerJob(_ string, _ any) error { return nil }

// NewNoop returns a Dispatcher that discards all dispatched work.
// Intended for use in tests.
func NewNoop() *Dispatcher {
	return New(noopTriggerer{})
}

// Dispatcher dispatches work to the background job runner.
type Dispatcher struct {
	runner triggerer
}

// New returns a Dispatcher backed by the given runner.
func New(runner triggerer) *Dispatcher {
	return &Dispatcher{runner: runner}
}

// Dispatch triggers a named job asynchronously.
// Returns an error if the name is not a registered job.
func (dispatcher *Dispatcher) Dispatch(jobName string, payload any) error {
	return dispatcher.runner.TriggerJob(jobName, payload)
}
