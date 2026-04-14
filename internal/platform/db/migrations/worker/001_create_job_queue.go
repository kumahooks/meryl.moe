package worker

const createJobQueue = `
	CREATE TABLE job_queue (
		id         TEXT    PRIMARY KEY,
		name       TEXT    NOT NULL,
		payload    TEXT,
		attempts   INTEGER NOT NULL DEFAULT 0,
		status     TEXT    NOT NULL DEFAULT 'pending',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)
`
