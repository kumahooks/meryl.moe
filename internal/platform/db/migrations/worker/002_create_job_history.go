package worker

const createJobHistory = `
	CREATE TABLE job_history (
		id           TEXT    PRIMARY KEY,
		name         TEXT    NOT NULL,
		payload      TEXT,
		attempts     INTEGER NOT NULL,
		completed_at INTEGER NOT NULL
	)
`
