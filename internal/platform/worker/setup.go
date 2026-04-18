package worker

import (
	"context"
	"database/sql"
	"time"

	kipplework "meryl.moe/internal/platform/worker/jobs/kipple"
	relaywork "meryl.moe/internal/platform/worker/jobs/relay"
)

// NewRegistrar builds and returns a Registrar with all background jobs registered.
// coreDatabase is used by job implementations (e.g. relay cleanup).
// workerDatabase is the dedicated job queue database.
// kippleDir is the directory where kipple files are stored on disk.
func NewRegistrar(coreDatabase *sql.DB, workerDatabase *sql.DB, kippleDir string) *Registrar {
	registrar := newRegistrar()
	registrar.workerDatabase = workerDatabase

	registrar.Register(Job{
		Name:     "relay:cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return relaywork.Cleanup(ctx, coreDatabase)
		},
	})

	registrar.Register(Job{
		Name:     "kipple:cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return kipplework.Cleanup(ctx, coreDatabase)
		},
	})

	registrar.Register(Job{
		Name:     "kipple:orphan-cleanup",
		Interval: 6 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return kipplework.CleanupOrphans(ctx, coreDatabase, kippleDir)
		},
	})

	return registrar
}
