package core

const createKippleFiles = `
	CREATE TABLE kipple_files (
		id          TEXT    PRIMARY KEY,
		user_id     TEXT    NOT NULL REFERENCES users(id),
		filename    TEXT    NOT NULL,
		size        INTEGER NOT NULL,
		offset      INTEGER NOT NULL DEFAULT 0,
		status      TEXT    NOT NULL DEFAULT 'pending',
		visibility  TEXT    NOT NULL DEFAULT 'link',
		expire_at   INTEGER NOT NULL,
		path        TEXT    NOT NULL,
		created_at  INTEGER NOT NULL
	)
`
