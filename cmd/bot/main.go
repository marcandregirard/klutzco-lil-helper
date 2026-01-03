package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	"klutco-lil-helper/internal/bot"
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

	b, err := bot.New(token, appId)
	if err != nil {
		log.Fatalf("failed to create bot: %v\n", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("bot stopped with error: %v\n", err)
	}
}
