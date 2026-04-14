// Package worker defines the ordered sequence of worker schema migrations.
// To add a migration: create a new file migration_NNN.go with a package-level
// const, then append it to the slice returned by All.
package worker

import (
	"meryl.moe/internal/platform/db/migrations"
)

// All returns all worker schema migrations in ascending ID order.
// Append only - never modify or reorder existing entries.
func All() []migrations.Migration {
	return []migrations.Migration{
		{ID: 1, Name: "001_create_job_queue", SQL: createJobQueue},
		{ID: 2, Name: "002_create_job_history", SQL: createJobHistory},
		{ID: 3, Name: "003_create_job_graveyard", SQL: createJobGraveyard},
	}
}
