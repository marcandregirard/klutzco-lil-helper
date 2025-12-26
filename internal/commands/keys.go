package commands

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"idle-clans-helper-bot/internal/model"

	"github.com/bwmarrin/discordgo"
)

var titleizer = cases.Title(language.Und)

var keysCommand = &discordgo.ApplicationCommand{
	Name:        "keys",
	Description: "Find a boss information by its key",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "name",
			Description:  "The name of the key you have.",
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

func keysHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "keys" {
		return
	}

	data := i.ApplicationCommandData()
	name := data.Options[0].StringValue()

	justForMe := false
	if len(data.Options) > 1 {
		justForMe = data.Options[1].BoolValue()
	}

	entry, ok := model.KeysInformation[strings.ToLower(name)]
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Unknown key: %s", name),
				Flags:   ephemeralFlag(justForMe),
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: name + " key",
		Description: fmt.Sprintf(
			"**%s**\nAttack style: 🛡️%s\nAttack style weakness: ⚔️%s",
			titleizer.String(entry.Name),
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

func keysAutocompleteHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return
	}
	if i.ApplicationCommandData().Name != "keys" {
		return
	}

	current := i.ApplicationCommandData().Options[0].StringValue()
	choices := []*discordgo.ApplicationCommandOptionChoice{}

	for key := range model.KeysInformation {
		if strings.Contains(strings.ToLower(key), strings.ToLower(current)) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  titleizer.String(key),
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
