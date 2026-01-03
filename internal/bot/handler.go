package bot

import (
	"klutco-lil-helper/internal/commands"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
}

func New(token string, appId string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuilds | discordgo.IntentsGuildMessages)

	b := &Bot{session: dg}
	commands.RegisterCommands(dg, appId)

	return b, nil
}

func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	log.Println("Bot is now running. Press CTRL-C to exit.")

	// Wait for interrupt signal to gracefully shut down
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down bot...")
	return b.session.Close()
}
