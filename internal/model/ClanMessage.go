package model

import (
	"database/sql"
	"time"
)

type ClanMessage struct {
	ClanName       string    `json:"clanName"`
	MemberUsername string    `json:"memberUsername"`
	Message        string    `json:"message"`
	Timestamp      time.Time `json:"timestamp"`
}

const createTableQuery = `
CREATE TABLE IF NOT EXISTS clan_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    clan_name TEXT NOT NULL,
    member_username TEXT NOT NULL,
    message TEXT NOT NULL,
    timestamp DATETIME NOT NULL
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

func GetMessagesByClan(db *sql.DB) ([]ClanMessage, error) {
	query := `
        SELECT clan_name, member_username, message, timestamp
        FROM clan_messages
        ORDER BY timestamp DESC
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

		if err := rows.Scan(
			&msg.ClanName,
			&msg.MemberUsername,
			&msg.Message,
			&ts,
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

		results = append(results, msg)
	}

	return results, nil
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(createTableQuery)
	return err
}
