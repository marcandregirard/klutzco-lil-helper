package model

import (
	"database/sql"
	"strings"
	"time"
)

type ClanMessage struct {
	ID             int64     `json:"id,omitempty"`
	ClanName       string    `json:"clanName"`
	MemberUsername string    `json:"memberUsername"`
	Message        string    `json:"message"`
	Timestamp      time.Time `json:"timestamp"`
	MessageSent    bool      `json:"messageSent"`
}

const createTableQuery = `
CREATE TABLE IF NOT EXISTS clan_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    clan_name TEXT NOT NULL,
    member_username TEXT NOT NULL,
    message TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    message_sent INTEGER NOT NULL DEFAULT 0
);

-- unique index to prevent duplicate entries (clan, member, message, timestamp)
CREATE UNIQUE INDEX IF NOT EXISTS idx_clan_messages_unique ON clan_messages (clan_name, member_username, message, timestamp);
`

func InsertClanMessage(db *sql.DB, msg ClanMessage) error {
	query := `
        INSERT OR IGNORE INTO clan_messages (clan_name, member_username, message, timestamp)
        VALUES (?, ?, ?, ?)
    `
	_, err := db.Exec(query,
		msg.ClanName,
		msg.MemberUsername,
		msg.Message,
		msg.Timestamp.UTC().Format(time.RFC3339),
	)
	return err
}

func GetMessages(db *sql.DB) ([]ClanMessage, error) {
	// return up to 10 oldest unsent messages
	query := `
        SELECT id, clan_name, member_username, message, timestamp, message_sent
        FROM clan_messages
        WHERE message_sent = 0
        ORDER BY timestamp ASC
        LIMIT 10
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ClanMessage

	for rows.Next() {
		var msg ClanMessage
		var ts string
		var sentInt int

		if err := rows.Scan(
			&msg.ID,
			&msg.ClanName,
			&msg.MemberUsername,
			&msg.Message,
			&ts,
			&sentInt,
		); err != nil {
			return nil, err
		}

		// parse timestamp stored as RFC3339
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			msg.Timestamp = t
		} else {
			// fallback: try parsing other common formats silently
			if t2, err2 := time.Parse("2006-01-02 15:04:05", ts); err2 == nil {
				msg.Timestamp = t2
			} else {
				// if parsing fails, leave zero time
				msg.Timestamp = time.Time{}
			}
		}

		msg.MessageSent = sentInt != 0

		results = append(results, msg)
	}

	return results, nil
}

// MarkMessagesSent marks the provided message IDs as sent (message_sent = 1).
func MarkMessagesSent(db *sql.DB, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	query := "UPDATE clan_messages SET message_sent = 1 WHERE id IN (" + placeholders + ")"

	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	_, err := db.Exec(query, args...)
	return err
}

// MigrateClanMessages handles migration for the clan_messages table.
// It creates the table if it doesn't exist, or upgrades from legacy schema if needed.
func MigrateClanMessages(db *sql.DB) error {
	// If table does not exist, create it using the canonical SQL
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='clan_messages'").Scan(&name)
	if err == sql.ErrNoRows {
		_, err := db.Exec(createTableQuery)
		return err
	}
	if err != nil {
		return err
	}

	// Table exists. Check whether `id` column is present. If present, ensure index exists and return.
	rows, err := db.Query("PRAGMA table_info(clan_messages)")
	if err != nil {
		return err
	}
	defer rows.Close()

	hasID := false
	for rows.Next() {
		var cid int
		var colName string
		var colType string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if colName == "id" {
			hasID = true
			break
		}
	}

	if hasID {
		// ensure unique index exists
		_, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_clan_messages_unique ON clan_messages (clan_name, member_username, message, timestamp)")
		return err
	}

	// Need to migrate existing table into new schema with `id` column.
	// Strategy: create new table, copy data, drop old table, rename new table, create index.
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// create new table
	_, err = tx.Exec(`
		CREATE TABLE clan_messages_new (
		    id INTEGER PRIMARY KEY AUTOINCREMENT,
		    clan_name TEXT NOT NULL,
		    member_username TEXT NOT NULL,
		    message TEXT NOT NULL,
		    timestamp DATETIME NOT NULL,
		    message_sent INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// copy data. If the old table lacks message_sent, COALESCE will default that to 0.
	_, err = tx.Exec(`
		INSERT INTO clan_messages_new (clan_name, member_username, message, timestamp, message_sent)
		SELECT clan_name, member_username, message, timestamp, COALESCE(message_sent, 0)
		FROM clan_messages
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// drop old table
	_, err = tx.Exec(`DROP TABLE clan_messages`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// rename new table
	_, err = tx.Exec(`ALTER TABLE clan_messages_new RENAME TO clan_messages`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// recreate unique index
	_, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_clan_messages_unique ON clan_messages (clan_name, member_username, message, timestamp)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
