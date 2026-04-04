package migrations

const createRoles = `
	CREATE TABLE roles (
		id          INTEGER PRIMARY KEY,
		name        TEXT    NOT NULL UNIQUE,
		permissions INTEGER NOT NULL DEFAULT 0
	)
`
