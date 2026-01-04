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

	db, err := sql.Open("sqlite", dbPath)
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

	b, err := bot.New(token, appId, db)
	if err != nil {
		log.Fatalf("failed to create bot: %v\n", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("bot stopped with error: %v\n", err)
	}
}
