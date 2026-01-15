package bot

import (
	"context"
	"database/sql"
	"klutco-lil-helper/internal/commands"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
	db      *sql.DB
}

func New(token string, appId string, db *sql.DB) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuilds | discordgo.IntentsGuildMessages)

	b := &Bot{session: dg, db: db}

	// Make DB available to command handlers
	commands.SetDB(db)

	commands.RegisterCommands(dg, appId)

	return b, nil
}

func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	log.Println("Bot is now running. Press CTRL-C to exit.")

	// start clan log fetcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	url := os.Getenv("CLAN_LOG_URL")
	interval := 24 * time.Hour
	if v := os.Getenv("CLAN_LOG_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}
	go b.runClanLogFetcher(ctx, interval, url)

	go b.runClanLogFetcher(ctx, 1*time.Minute, "https://query.idleclans.com/api/Clan/logs/clan/KlutzCo?limit=10")

	// start message sender (every 30s)
	channelName := os.Getenv("CLAN_MESSAGE_CHANNEL")
	go b.runMessageSender(ctx, channelName)

	// start boss scheduler (posts to channel named by BOSS_CHANNEL, default "boss")
	bossChannel := os.Getenv("BOSS_CHANNEL")
	if bossChannel == "" {
		bossChannel = "boss"
	}
	go b.runBossScheduler(ctx, bossChannel)

	// Wait for interrupt signal to gracefully shut down
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down bot...")
	// Close discord session first, then DB
	err := b.session.Close()
	if b.db != nil {
		_ = b.db.Close()
	}
	return err
}
