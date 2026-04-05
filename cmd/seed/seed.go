package main

import (
	"database/sql"
	"fmt"
)

func seed(database *sql.DB) error {
	transaction, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer transaction.Rollback()

	if err := seedRoles(transaction); err != nil {
		return err
	}

	if err := seedUsers(transaction); err != nil {
		return err
	}

	return transaction.Commit()
}
