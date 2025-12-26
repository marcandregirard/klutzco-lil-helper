package commands

import (
	"fmt"
	"strings"

	"idle-clans-helper-bot/internal/model"

	"github.com/bwmarrin/discordgo"
)

var bossCommand = &discordgo.ApplicationCommand{
	Name:        "boss",
	Description: "Find a boss information by its name",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "name",
			Description:  "The name of the boss to find.",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "just_for_me",
			Description: "Only show the definition to me.",
			Required:    false,
		},
	},
}

func bossHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "boss" {
		return
	}

	data := i.ApplicationCommandData()
	name := data.Options[0].StringValue()

	justForMe := false
	if len(data.Options) > 1 {
		justForMe = data.Options[1].BoolValue()
	}

	entry, ok := model.BossesInformation[strings.ToLower(name)]
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Unknown boss: %s", name),
				Flags:   ephemeralFlag(justForMe),
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: titleizer.String(name),
		Description: fmt.Sprintf(
			"Key needed: **%s**\nAttack style: 🛡️%s\nAttack style weakness: ⚔️%s",
			titleizer.String(entry.Key),
			entry.AttackStyle,
			entry.AttackWeakness,
		),
		URL:   "https://wiki.idleclans.com/index.php/" + entry.Wiki,
		Color: entry.TrimColor,
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  ephemeralFlag(justForMe),
		},
	})
}

func bossAutocompleteHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return
	}
	if i.ApplicationCommandData().Name != "boss" {
		return
	}

	current := i.ApplicationCommandData().Options[0].StringValue()
	choices := []*discordgo.ApplicationCommandOptionChoice{}

	for key := range model.BossesInformation {
		if strings.Contains(strings.ToLower(key), strings.ToLower(current)) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  strings.Title(key),
				Value: key,
			})
		}
		if len(choices) >= 25 {
			break
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}
