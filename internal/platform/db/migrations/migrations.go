// Package migrations defines the ordered sequence of database schema migrations.
// To add a migration: create a new file migration_NNN.go with a package-level
// const, then append it to the slice returned by All.
package migrations

// Migration represents a single versioned schema change.
type Migration struct {
	ID   int
	Name string
	SQL  string
}

// All returns all migrations in ascending ID order.
// Append only - never modify or reorder existing entries.
func All() []Migration {
	return []Migration{
		{1, "001_create_users", createUsers},
		{2, "002_create_sessions", createSessions},
		{3, "003_create_relays", createRelays},
		{4, "004_create_roles", createRoles},
		{5, "005_create_users_roles", createUsersRoles},
		{6, "006_add_relay_private_mode", addRelayPrivateMode},
		{7, "007_add_relay_expire_at", addRelayExpireAt},
		{8, "008_drop_relay_private", dropRelayPrivate},
	}
}
