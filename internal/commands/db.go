package commands

import "database/sql"

// DB is the package-level database handle used by command handlers.
var DB *sql.DB

// SetDB stores the opened database connection for command handlers to use.
func SetDB(db *sql.DB) {
	DB = db
}
