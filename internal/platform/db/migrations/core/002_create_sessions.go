package core

const createSessions = `
	CREATE TABLE sessions (
		token_hash TEXT    PRIMARY KEY,
		user_id    TEXT    NOT NULL REFERENCES users(id),
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	)
`
