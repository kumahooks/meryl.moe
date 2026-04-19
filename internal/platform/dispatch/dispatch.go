// Package dispatch provides a work dispatcher for triggering background jobs
// from within module handlers without direct coupling to the worker package.
package dispatch

// Job name constants exposed to Dispatch
const (
	RelayCleanup        = "relay:cleanup"
	KippleCleanup       = "kipple:cleanup"
	KippleOrphanCleanup = "kipple:orphan_cleanup"
)

type enqueuer interface {
	Enqueue(jobName string, payload any) error
}

type noopEnqueuer struct{}

func (noopEnqueuer) Enqueue(_ string, _ any) error { return nil }

// NewNoop returns a Dispatcher that discards all dispatched work.
// Intended for use in tests.
func NewNoop() *Dispatcher {
	return New(noopEnqueuer{})
}

// Dispatcher dispatches work to the background job runner.
type Dispatcher struct {
	runner enqueuer
}

// New returns a Dispatcher backed by the given runner.
func New(runner enqueuer) *Dispatcher {
	return &Dispatcher{runner: runner}
}

// Dispatch enqueues a named job for asynchronous execution.
// Returns an error if the name is not a registered job.
func (dispatcher *Dispatcher) Dispatch(jobName string, payload any) error {
	return dispatcher.runner.Enqueue(jobName, payload)
}
