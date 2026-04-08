// Package worker provides a simple background job runner.
// Jobs are registered with a name and interval and run concurrently.
// Each job runs immediately on start, then repeats at its interval.
// A job with Interval == 0 is on-demand only and never runs automatically.
// A job that returns an error is logged but not stopped.
package worker

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Job is a named, recurring background task.
// Interval = 0 is for on-demand-only jobs that are never scheduled automatically.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context, payload any) error
}

// Registrar collects jobs before the job runner is built.
// Call Register on each work package, then build the JobRunner via JobRunner().
type Registrar struct {
	jobs []Job
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
	return &JobRunner{jobs: registrar.jobs}
}

// JobRunner holds a set of jobs and runs them concurrently.
type JobRunner struct {
	jobs   []Job
	jobMap map[string]Job
}

// Start spawns one goroutine per scheduled job (Interval > 0). Each goroutine
// runs its job immediately, then at every Job.Interval. On-demand jobs
// (Interval == 0) are registered but never run automatically.
// All goroutines exit when ctx is cancelled.
func (jobRunner *JobRunner) Start(ctx context.Context) {
	jobRunner.jobMap = make(map[string]Job, len(jobRunner.jobs))

	log.Printf("worker: initialized (%d job(s))", len(jobRunner.jobs))

	for _, job := range jobRunner.jobs {
		jobRunner.jobMap[job.Name] = job

		if job.Interval > 0 {
			go runJob(ctx, job)
		}
	}
}

// TriggerJob spawns a one-off run of the named job asynchronously.
// Returns an error if the name is not registered.
func (jobRunner *JobRunner) TriggerJob(name string, payload any) error {
	job, ok := jobRunner.jobMap[name]
	if !ok {
		return fmt.Errorf("worker: unknown job: %s", name)
	}

	go runOnce(context.Background(), job, payload)

	return nil
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
