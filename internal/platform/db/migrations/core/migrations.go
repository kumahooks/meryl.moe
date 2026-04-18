// Package core defines the ordered sequence of core schema migrations.
// To add a migration: create a new file migration_NNN.go with a package-level
// const, then append it to the slice returned by All.
package core

import (
	"meryl.moe/internal/platform/db/migrations"
)

// All returns all core schema migrations in ascending ID order.
// Append only - never modify or reorder existing entries.
func All() []migrations.Migration {
	return []migrations.Migration{
		{ID: 1, Name: "001_create_users", SQL: createUsers},
		{ID: 2, Name: "002_create_sessions", SQL: createSessions},
		{ID: 3, Name: "003_create_relays", SQL: createRelays},
		{ID: 4, Name: "004_create_roles", SQL: createRoles},
		{ID: 5, Name: "005_create_users_roles", SQL: createUsersRoles},
		{ID: 6, Name: "006_add_relay_private_mode", SQL: addRelayPrivateMode},
		{ID: 7, Name: "007_add_relay_expire_at", SQL: addRelayExpireAt},
		{ID: 8, Name: "008_drop_relay_private", SQL: dropRelayPrivate},
		{ID: 9, Name: "009_create_kipple_files", SQL: createKippleFiles},
	}
}
