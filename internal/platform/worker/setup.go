package worker

import (
	"context"
	"database/sql"
	"time"

	authwork "meryl.moe/internal/platform/worker/jobs/auth"
	relaywork "meryl.moe/internal/platform/worker/jobs/relay"
)

// NewRegistrar builds and returns a Registrar with all background jobs registered.
// coreDatabase is used by job implementations (e.g. relay cleanup).
// workerDatabase is the dedicated job queue database.
func NewRegistrar(coreDatabase *sql.DB, workerDatabase *sql.DB) *Registrar {
	registrar := newRegistrar()
	registrar.workerDatabase = workerDatabase

	registrar.Register(Job{
		Name:     "relay:cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return relaywork.Cleanup(ctx, coreDatabase)
		},
	})

	// TODO: remove - test job to verify dispatch mechanism
	registrar.Register(Job{
		Name: "auth:login",
		Run:  authwork.LogLogin,
	})

	return registrar
}
