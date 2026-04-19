// Package relay provides the background cleanup logic for expired relays.
package relay

import (
	"context"
	"database/sql"
	"log"
)

// Cleanup deletes all expired relays from the database.
func Cleanup(ctx context.Context, database *sql.DB) error {
	deleted, err := deleteExpired(ctx, database)
	if err != nil {
		return err
	}

	if deleted > 0 {
		log.Printf("[relay:job] cleanup: deleted %d expired relay(s)", deleted)
	}

	return nil
}
