package commands

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func RegisterCommands(s *discordgo.Session, appId string) {
	// Register slash commands
	registerCommand(s, bossCommand, appId)
	registerCommand(s, keysCommand, appId)
	registerCommand(s, marketFoodCommand, appId)
	registerCommand(s, bossSummaryCommand, appId)

	// Register handlers
	s.AddHandler(bossHandler)
	s.AddHandler(bossAutocompleteHandler)

	s.AddHandler(keysHandler)
	s.AddHandler(keysAutocompleteHandler)

	s.AddHandler(marketFoodHandler)

	s.AddHandler(bossSummaryHandler)
}

func registerCommand(s *discordgo.Session, cmd *discordgo.ApplicationCommand, appId string) {
	_, err := s.ApplicationCommandCreate(appId, "", cmd)
	if err != nil {
		log.Printf("Failed to register command %s: %v", cmd.Name, err)
	}
}

// Helper for ephemeral messages
func ephemeralFlag(ephemeral bool) discordgo.MessageFlags {
	if ephemeral {
		return discordgo.MessageFlagsEphemeral
	}
	return 0
}
