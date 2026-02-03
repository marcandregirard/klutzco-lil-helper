package model

import (
	"database/sql"
	"time"
)

// MessageType represents the type of scheduled message
type MessageType string

const (
	MessageTypeDaily       MessageType = "daily"
	MessageTypeWeekly      MessageType = "weekly"
	MessageTypeBossSummary MessageType = "bosssummary"
)

// ScheduledMessage represents a scheduled message stored in the database
// for tracking and deletion purposes.
type ScheduledMessage struct {
	Type      MessageType `json:"type"` // MessageTypeDaily or MessageTypeWeekly
	ChannelID string      `json:"channelId"`
	MessageID string      `json:"messageId"`
	CreatedAt time.Time   `json:"createdAt"`
}

const createScheduledMessagesTableQuery = `
CREATE TABLE IF NOT EXISTS scheduled_messages (
    type TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (type, channel_id)
);
`

// CreateScheduledMessagesTable creates the scheduled_messages table if it does not exist.
func CreateScheduledMessagesTable(db *sql.DB) error {
	_, err := db.Exec(createScheduledMessagesTableQuery)
	return err
}

// UpsertScheduledMessage inserts or updates a scheduled message record.
// This uses SQLite's INSERT OR REPLACE to handle both insert and update cases.
func UpsertScheduledMessage(db *sql.DB, msgType MessageType, channelID, messageID string) error {
	query := `
		INSERT OR REPLACE INTO scheduled_messages (type, channel_id, message_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := db.Exec(query, msgType, channelID, messageID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// GetScheduledMessage retrieves the message ID for a given type and channel.
// Returns empty string and nil error if no record exists.
func GetScheduledMessage(db *sql.DB, msgType MessageType, channelID string) (string, error) {
	query := `
		SELECT message_id FROM scheduled_messages
		WHERE type = ? AND channel_id = ?
	`
	var messageID string
	err := db.QueryRow(query, msgType, channelID).Scan(&messageID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return messageID, nil
}

// DeleteScheduledMessage removes a scheduled message record from the database.
func DeleteScheduledMessage(db *sql.DB, msgType MessageType, channelID string) error {
	query := `DELETE FROM scheduled_messages WHERE type = ? AND channel_id = ?`
	_, err := db.Exec(query, msgType, channelID)
	return err
}
