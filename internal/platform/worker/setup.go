package worker

import (
	"context"
	"database/sql"
	"time"

	authwork "meryl.moe/internal/platform/worker/works/auth"
	relaywork "meryl.moe/internal/platform/worker/works/relay"
)

// NewRegistrar builds and returns a Registrar with all background jobs registered.
func NewRegistrar(database *sql.DB) *Registrar {
	registrar := newRegistrar()

	registrar.Register(Job{
		Name:     "relay:cleanup",
		Interval: 1 * time.Hour,
		Run: func(ctx context.Context, _ any) error {
			return relaywork.Cleanup(ctx, database)
		},
	})

	// TODO: remove - test job to verify dispatch mechanism
	registrar.Register(Job{
		Name: "auth:login",
		Run:  authwork.LogLogin,
	})

	return registrar
}
