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
