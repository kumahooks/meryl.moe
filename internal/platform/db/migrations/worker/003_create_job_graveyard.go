package worker

const createJobGraveyard = `
	CREATE TABLE job_graveyard (
		id         TEXT    PRIMARY KEY,
		name       TEXT    NOT NULL,
		payload    TEXT,
		attempts   INTEGER NOT NULL,
		last_error TEXT    NOT NULL,
		buried_at  INTEGER NOT NULL
	)
`
