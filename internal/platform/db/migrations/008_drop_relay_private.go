package migrations

const dropRelayPrivate = `
	ALTER TABLE relays DROP COLUMN private
`
