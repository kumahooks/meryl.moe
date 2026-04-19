package worker

import (
	"context"
	"database/sql"
	"time"

	kipplework "meryl.moe/internal/platform/worker/jobs/kipple"
	loggingwork "meryl.moe/internal/platform/worker/jobs/logging"
	relaywork "meryl.moe/internal/platform/worker/jobs/relay"
)

// NewRegistrar builds and returns a Registrar with all background jobs registered.
// coreDatabase is used by job implementations (e.g. relay cleanup).
// workerDatabase is the dedicated job queue database.
// kippleDir is the directory where kipple files are stored on disk.
// logDir is the directory where application log files are written.
func NewRegistrar(coreDatabase *sql.DB, workerDatabase *sql.DB, kippleDir string, logDir string) *Registrar {
	registrar := newRegistrar()
	registrar.workerDatabase = workerDatabase

	registrar.Register(Job{
		Name:     "relay:cleanup",
		Interval: 30 * time.Minute,
		Run: func(ctx context.Context, _ any) error {
			return relaywork.Cleanup(ctx, coreDatabase)
		},
	})

	registrar.Register(Job{
		Name:     "kipple:cleanup",
		Interval: 30 * time.Minute,
		Run: func(ctx context.Context, _ any) error {
			return kipplework.Cleanup(ctx, coreDatabase)
		},
	})

	registrar.Register(Job{
		Name:     "kipple:orphan_cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return kipplework.CleanupOrphans(ctx, coreDatabase, kippleDir)
		},
	})

	registrar.Register(Job{
		Name:     "logging:cleanup",
		Interval: 24 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return loggingwork.Cleanup(ctx, logDir)
		},
	})

	return registrar
}
