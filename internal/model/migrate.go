package model

import "database/sql"

// Migrate runs all database migrations in order.
// This is the single entry point for database setup.
func Migrate(db *sql.DB) error {
	// Migrate clan_messages table (handles both fresh and legacy upgrades)
	if err := MigrateClanMessages(db); err != nil {
		return err
	}

	// Create scheduled_messages table
	if err := CreateScheduledMessagesTable(db); err != nil {
		return err
	}

	return nil
}
