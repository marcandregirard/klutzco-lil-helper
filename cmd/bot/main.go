package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"

	"klutco-lil-helper/internal/bot"
	"klutco-lil-helper/internal/model"
)

func main() {
	// Load .env if present (local dev)
	_ = godotenv.Load()

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatalln("DISCORD_BOT_TOKEN is not set")
	}

	appId := os.Getenv("DISCORD_APP_ID")
	if appId == "" {
		log.Fatalln("DISCORD_APP_ID is not set")
	}
	// Open (or create) the sqlite database. DB_PATH env var can override the default.
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		// default location inside container where docker-compose will mount volume
		dbPath = "/app/data/lilhelper.db"
	}

	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		log.Fatalln("failed to open database:", err)
	}

	// Verify we can connect/open the file
	if err := db.Ping(); err != nil {
		log.Fatalln("failed to ping database", err)
	}

	// Run migrations
	if err := model.Migrate(db); err != nil {
		log.Fatalln("failed to migrate database:", err)
	}
	// Important: limit to 1 connection for sqlite to avoid writer/reader connection churn.
	db.SetMaxOpenConns(1)

	// Optional tuning
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(1)

	// Ensure PRAGMAs applied on open connection (safe to ignore error on Exec if driver already set)
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	_, _ = db.Exec("PRAGMA synchronous = NORMAL;")

	b, err := bot.New(token, appId, db)
	if err != nil {
		log.Fatalf("failed to create bot: %v\n", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("bot stopped with error: %v\n", err)
	}
}
