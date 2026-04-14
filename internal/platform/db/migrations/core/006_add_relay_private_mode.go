package core

const addRelayPrivateMode = `
	ALTER TABLE relays ADD COLUMN private_mode TEXT NOT NULL DEFAULT 'link'
`
