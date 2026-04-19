package kipple

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func deleteExpired(ctx context.Context, database *sql.DB) (int64, error) {
	now := time.Now().Unix()
	// 3600 seconds; pending uploads older than this are considered abandoned
	staleThreshold := now - 3600

	rows, err := database.QueryContext(ctx,
		`SELECT id, path FROM kipple_files
		 WHERE (status = 'complete' AND expire_at <= ?) OR (status = 'pending' AND created_at <= ?)`,
		now, staleThreshold,
	)
	if err != nil {
		return 0, fmt.Errorf("[kipple:job] query expired kipple files: %w", err)
	}

	defer rows.Close()

	type entry struct{ id, path string }

	var entries []entry

	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.path); err != nil {
			return 0, fmt.Errorf("[kipple:job] scan: %w", err)
		}

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("[kipple:job] iterate: %w", err)
	}

	var deleted int64

	for _, e := range entries {
		result, err := database.ExecContext(ctx, "DELETE FROM kipple_files WHERE id = ?", e.id)
		if err != nil {
			return deleted, fmt.Errorf("[kipple:job] delete kipple file %s: %w", e.id, err)
		}

		n, _ := result.RowsAffected()
		deleted += n

		os.Remove(e.path)
	}

	return deleted, nil
}

// deleteOrphaned removes DB rows whose disk file is missing and disk files with no DB row.
// Returns counts of DB rows and disk files deleted.
func deleteOrphaned(ctx context.Context, database *sql.DB, dir string) (int64, int64, error) {
	rows, err := database.QueryContext(ctx, "SELECT id, path FROM kipple_files")
	if err != nil {
		return 0, 0, fmt.Errorf("[kipple:job] query kipple files: %w", err)
	}

	defer rows.Close()

	type entry struct{ id, path string }

	var entries []entry
	knownPaths := make(map[string]struct{})

	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.id, &e.path); err != nil {
			return 0, 0, fmt.Errorf("scan: %w", err)
		}

		entries = append(entries, e)
		knownPaths[e.path] = struct{}{}
	}

	if err = rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("[kipple:job] iterate: %w", err)
	}

	var dbOrphans int64

	for _, e := range entries {
		if _, statErr := os.Stat(e.path); os.IsNotExist(statErr) {
			result, dbErr := database.ExecContext(ctx, "DELETE FROM kipple_files WHERE id = ?", e.id)
			if dbErr != nil {
				return dbOrphans, 0, fmt.Errorf("[kipple:job] delete orphaned row %s: %w", e.id, dbErr)
			}

			n, _ := result.RowsAffected()
			dbOrphans += n
		}
	}

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dbOrphans, 0, nil
		}

		return dbOrphans, 0, fmt.Errorf("[kipple:job] read kipple dir: %w", err)
	}

	var diskOrphans int64

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dir, dirEntry.Name())
		if _, known := knownPaths[fullPath]; !known {
			if os.Remove(fullPath) == nil {
				diskOrphans++
			}
		}
	}

	return dbOrphans, diskOrphans, nil
}
