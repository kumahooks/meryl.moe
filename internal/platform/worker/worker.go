// Package worker provides a simple background job runner.
// Scheduled jobs (Interval > 0) run immediately on start, then repeat at their interval.
// On-demand jobs (Interval == 0) are enqueued to the DB queue via Enqueue and executed
// by the poll loop, with automatic retry and graveyard on repeated failure.
package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

const pollInterval = 5 * time.Second

// Job is a named background task.
// Interval = 0 is for on-demand jobs that are dispatched via the DB queue.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context, payload any) error
}

// Registrar collects jobs before the job runner is built.
// Call Register on each work package, then build the JobRunner via JobRunner().
type Registrar struct {
	jobs           []Job
	workerDatabase *sql.DB
}

func newRegistrar() *Registrar {
	return &Registrar{}
}

// Register appends one or more jobs to the registrar.
func (registrar *Registrar) Register(jobs ...Job) {
	for _, job := range jobs {
		log.Printf("worker: registered: %s", job.Name)
		registrar.jobs = append(registrar.jobs, job)
	}
}

// JobRunner builds a JobRunner from all registered jobs.
func (registrar *Registrar) JobRunner() *JobRunner {
	jobMap := make(map[string]Job, len(registrar.jobs))
	for _, job := range registrar.jobs {
		jobMap[job.Name] = job
	}

	return &JobRunner{
		jobs:   registrar.jobs,
		jobMap: jobMap,
		queue:  &jobQueue{database: registrar.workerDatabase},
	}
}

// JobRunner holds a set of jobs and runs them concurrently.
type JobRunner struct {
	jobs   []Job
	jobMap map[string]Job
	queue  *jobQueue
}

// Start spawns one goroutine per scheduled job (Interval > 0). Each goroutine
// runs its job immediately, then at every Job.Interval. On-demand jobs
// (Interval == 0) are executed via the DB queue poll loop.
// All goroutines exit when ctx is cancelled.
func (jobRunner *JobRunner) Start(ctx context.Context) {
	log.Printf("worker: initialized (%d job(s))", len(jobRunner.jobs))

	for _, job := range jobRunner.jobs {
		if job.Interval > 0 {
			go runJob(ctx, job)
		}
	}

	if jobRunner.queue.database != nil {
		go jobRunner.pollLoop(ctx)
	}
}

// Enqueue writes a job to the DB queue for asynchronous execution.
// Returns an error if the name is not registered.
func (jobRunner *JobRunner) Enqueue(name string, payload any) error {
	if _, ok := jobRunner.jobMap[name]; !ok {
		return fmt.Errorf("worker: unknown job: %s", name)
	}

	return jobRunner.queue.enqueue(name, payload)
}

// TriggerJob spawns a one-off run of the named job directly, bypassing the DB queue.
// Used for tests and internal use. Returns an error if the name is not registered.
func (jobRunner *JobRunner) TriggerJob(name string, payload any) error {
	job, ok := jobRunner.jobMap[name]
	if !ok {
		return fmt.Errorf("worker: unknown job: %s", name)
	}

	go runOnce(context.Background(), job, payload)

	return nil
}

func (jobRunner *JobRunner) pollLoop(ctx context.Context) {
	jobRunner.queue.resetOrphaned(ctx)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			jobRunner.processQueue(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (jobRunner *JobRunner) processQueue(ctx context.Context) {
	records, err := jobRunner.queue.claimPending(ctx)
	if err != nil {
		log.Printf("worker: queue: poll: %v", err)
		return
	}

	for _, record := range records {
		go jobRunner.executeQueued(ctx, record)
	}
}

func (jobRunner *JobRunner) executeQueued(ctx context.Context, record jobRecord) {
	job, ok := jobRunner.jobMap[record.name]
	if !ok {
		log.Printf("worker: queue: %s: unknown job (id=%s)", record.name, record.id)

		if err := jobRunner.queue.bury(
			record,
			record.attempts,
			fmt.Sprintf("unknown job: %s", record.name),
		); err != nil {
			log.Printf("worker: queue: %s: bury: %v", record.name, err)
		}

		return
	}

	var payload any
	if record.payload != nil {
		if err := json.Unmarshal([]byte(*record.payload), &payload); err != nil {
			log.Printf("worker: queue: %s: unmarshal payload: %v (id=%s)", record.name, err, record.id)

			if err = jobRunner.queue.bury(record, record.attempts, err.Error()); err != nil {
				log.Printf("worker: queue: %s: bury: %v", record.name, err)
			}

			return
		}
	}

	newAttempts := record.attempts + 1
	log.Printf("worker: queue: %s: started (attempt %d/%d)", record.name, newAttempts, maxAttempts)

	start := time.Now()

	runErr := job.Run(ctx, payload)
	if runErr == nil {
		log.Printf("worker: queue: %s: done (%s)", record.name, time.Since(start))

		if err := jobRunner.queue.complete(record, newAttempts); err != nil {
			log.Printf("worker: queue: %s: complete: %v", record.name, err)
		}

		return
	}

	log.Printf("worker: queue: %s: attempt %d failed: %v", record.name, newAttempts, runErr)

	if newAttempts >= maxAttempts {
		log.Printf("worker: queue: %s: max attempts reached, burying (id=%s)", record.name, record.id)

		if err := jobRunner.queue.bury(record, newAttempts, runErr.Error()); err != nil {
			log.Printf("worker: queue: %s: bury: %v", record.name, err)
		}

		return
	}

	if err := jobRunner.queue.retry(record.id, newAttempts); err != nil {
		log.Printf("worker: queue: %s: retry: %v", record.name, err)
	}
}

func runJob(ctx context.Context, job Job) {
	runOnce(ctx, job, nil)

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runOnce(ctx, job, nil)
		case <-ctx.Done():
			return
		}
	}
}

func runOnce(ctx context.Context, job Job, payload any) {
	log.Printf("worker: %s: started", job.Name)
	start := time.Now()

	if err := job.Run(ctx, payload); err != nil {
		log.Printf("worker: %s: %v", job.Name, err)
		return
	}

	log.Printf("worker: %s: done (%s)", job.Name, time.Since(start))
}
