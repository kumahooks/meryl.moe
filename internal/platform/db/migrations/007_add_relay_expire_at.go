package migrations

const addRelayExpireAt = `
	ALTER TABLE relays ADD COLUMN expire_at INTEGER NOT NULL DEFAULT 0
`
