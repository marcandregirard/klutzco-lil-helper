package commands

import (
	"log"
	"os"

	"klutco-lil-helper/internal/bosssummary"
	"klutco-lil-helper/internal/model"

	"github.com/bwmarrin/discordgo"
)

var bossSummaryCommand = &discordgo.ApplicationCommand{
	Name:        "boss_summary",
	Description: "Regenerate the boss summary message",
}

func bossSummaryHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "boss_summary" {
		return
	}

	// Acknowledge the interaction immediately
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("[boss_summary] failed to acknowledge interaction: %v", err)
		return
	}

	// Find the configured summary channel (not the channel where command was invoked)
	summaryChannelName := os.Getenv("BOSS_SUMMARY_CHANNEL")
	if summaryChannelName == "" {
		summaryChannelName = "tactical-dispatch"
	}

	// Look up the summary channel ID
	var summaryChannelID string
	if s.State != nil {
		for _, guild := range s.State.Guilds {
			for _, channel := range guild.Channels {
				if channel.Name == summaryChannelName && channel.Type == discordgo.ChannelTypeGuildText {
					summaryChannelID = channel.ID
					break
				}
			}
			if summaryChannelID != "" {
				break
			}
		}
	}

	if summaryChannelID == "" {
		log.Printf("[boss_summary] summary channel %q not found", summaryChannelName)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("Failed to find summary channel."),
		})
		return
	}

	// Check if there's an existing boss summary message in the summary channel
	summaryMsgID, err := model.GetScheduledMessage(DB, model.MessageTypeBossSummary, summaryChannelID)
	if err != nil {
		log.Printf("[boss_summary] failed to get scheduled message: %v", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("Failed to check for existing summary message."),
		})
		return
	}

	if summaryMsgID == "" {
		// No existing summary, do nothing
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("No boss summary found to regenerate."),
		})
		return
	}

	// Regenerate the summary in the configured summary channel
	err = bosssummary.RegenerateSummary(s, DB, summaryChannelID)
	if err != nil {
		log.Printf("[boss_summary] failed to regenerate summary: %v", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: strPtr("Failed to regenerate boss summary."),
		})
		return
	}

	// Confirm success
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: strPtr("Boss summary has been regenerated."),
	})
}

func strPtr(s string) *string {
	return &s
}
