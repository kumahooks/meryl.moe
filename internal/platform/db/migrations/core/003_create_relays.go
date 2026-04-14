package core

const createRelays = `
	CREATE TABLE relays (
		id         TEXT    PRIMARY KEY,
		user_id    TEXT    NOT NULL REFERENCES users(id),
		content    BLOB    NOT NULL,
		private    INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	)
`
