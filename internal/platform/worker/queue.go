package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

const maxAttempts = 3

type jobRecord struct {
	id       string
	name     string
	payload  *string
	attempts int
}

type jobQueue struct {
	database *sql.DB
}

// enqueue inserts a new pending job into the queue.
func (queue *jobQueue) enqueue(name string, payload any) error {
	var encodedPayload *string
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		serialized := string(encoded)
		encodedPayload = &serialized
	}

	now := time.Now().Unix()

	if _, err := queue.database.Exec(
		"INSERT INTO job_queue (id, name, payload, attempts, status, created_at, updated_at) VALUES (?, ?, ?, 0, 'pending', ?, ?)",
		uuid.New().String(),
		name,
		encodedPayload,
		now,
		now,
	); err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	return nil
}

// claimPending marks all pending jobs as running and returns them.
func (queue *jobQueue) claimPending(ctx context.Context) ([]jobRecord, error) {
	rows, err := queue.database.QueryContext(
		ctx,
		"UPDATE job_queue SET status = 'running', updated_at = ? WHERE status = 'pending' RETURNING id, name, payload, attempts",
		time.Now().Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("claim pending: %w", err)
	}

	defer rows.Close()

	var records []jobRecord
	for rows.Next() {
		var record jobRecord
		if err := rows.Scan(&record.id, &record.name, &record.payload, &record.attempts); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}

	return records, nil
}

// complete moves a job from the queue to job_history.
func (queue *jobQueue) complete(record jobRecord, attempts int) error {
	transaction, err := queue.database.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if _, err := transaction.Exec(
		"INSERT INTO job_history (id, name, payload, attempts, completed_at) VALUES (?, ?, ?, ?, ?)",
		record.id, record.name, record.payload, attempts, time.Now().Unix(),
	); err != nil {
		transaction.Rollback()
		return fmt.Errorf("insert history: %w", err)
	}

	if _, err := transaction.Exec("DELETE FROM job_queue WHERE id = ?", record.id); err != nil {
		transaction.Rollback()
		return fmt.Errorf("delete from queue: %w", err)
	}

	return transaction.Commit()
}

// retry resets a job to pending and updates its attempt count.
func (queue *jobQueue) retry(id string, attempts int) error {
	if _, err := queue.database.Exec(
		"UPDATE job_queue SET status = 'pending', attempts = ?, updated_at = ? WHERE id = ?",
		attempts, time.Now().Unix(), id,
	); err != nil {
		return fmt.Errorf("retry job: %w", err)
	}

	return nil
}

// bury moves a job from the queue to job_graveyard after exceeding max attempts.
func (queue *jobQueue) bury(record jobRecord, attempts int, lastError string) error {
	transaction, err := queue.database.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if _, err := transaction.Exec(
		"INSERT INTO job_graveyard (id, name, payload, attempts, last_error, buried_at) VALUES (?, ?, ?, ?, ?, ?)",
		record.id, record.name, record.payload, attempts, lastError, time.Now().Unix(),
	); err != nil {
		transaction.Rollback()
		return fmt.Errorf("insert graveyard: %w", err)
	}

	if _, err := transaction.Exec("DELETE FROM job_queue WHERE id = ?", record.id); err != nil {
		transaction.Rollback()
		return fmt.Errorf("delete from queue: %w", err)
	}

	return transaction.Commit()
}

// resetOrphaned resets any jobs stuck in 'running' state back to 'pending'.
// Called on startup to recover from unclean shutdowns.
func (queue *jobQueue) resetOrphaned(ctx context.Context) {
	result, err := queue.database.ExecContext(ctx,
		"UPDATE job_queue SET status = 'pending', updated_at = ? WHERE status = 'running'",
		time.Now().Unix(),
	)
	if err != nil {
		log.Printf("worker: queue: reset orphaned: %v", err)
		return
	}

	if affected, _ := result.RowsAffected(); affected > 0 {
		log.Printf("worker: queue: reset %d orphaned job(s)", affected)
	}
}
