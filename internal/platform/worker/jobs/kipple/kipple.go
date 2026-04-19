// Package kipple provides the background cleanup logic for expired kipple files.
package kipple

import (
	"context"
	"database/sql"
	"log"
)

// Cleanup deletes expired completed files and stale pending uploads from the database and disk.
func Cleanup(ctx context.Context, database *sql.DB) error {
	deleted, err := deleteExpired(ctx, database)
	if err != nil {
		return err
	}

	if deleted > 0 {
		log.Printf("[kipple:job] cleanup: deleted %d expired file(s)", deleted)
	}

	return nil
}

// CleanupOrphans removes DB rows whose disk file is missing and disk files with no DB row.
func CleanupOrphans(ctx context.Context, database *sql.DB, dir string) error {
	dbOrphans, diskOrphans, err := deleteOrphaned(ctx, database, dir)
	if err != nil {
		return err
	}

	if dbOrphans > 0 {
		log.Printf("[kipple:job] orphan cleanup: removed %d DB row(s) with missing disk file", dbOrphans)
	}

	if diskOrphans > 0 {
		log.Printf("[kipple:job] orphan cleanup: removed %d disk file(s) with no DB row", diskOrphans)
	}

	return nil
}
