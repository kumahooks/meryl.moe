package migrations

const createUsers = `
	CREATE TABLE users (
		id            TEXT    PRIMARY KEY,
		username      TEXT    NOT NULL UNIQUE,
		password_hash TEXT    NOT NULL,
		last_login_at INTEGER,
		updated_at    INTEGER NOT NULL,
		created_at    INTEGER NOT NULL
	)
`
