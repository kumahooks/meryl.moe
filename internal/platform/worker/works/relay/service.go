package relay

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func deleteExpired(ctx context.Context, database *sql.DB) (int64, error) {
	result, err := database.ExecContext(ctx,
		"DELETE FROM relays WHERE expire_at <= ?",
		time.Now().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired relays: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return deleted, nil
}
