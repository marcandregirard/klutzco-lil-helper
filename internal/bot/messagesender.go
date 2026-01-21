package bot

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"time"
	_ "time/tzdata" // Embed timezone database for containerized environments

	"klutco-lil-helper/internal/model"

	"github.com/bwmarrin/discordgo"
)

const defaultPendingChannel = "testing-ground"
const defaultDonationChannel = "general"

var memberToDiscord = map[string]string{
	"ImaKlutz":  "ImaKlutz",
	"guildan":   "Guildan",
	"Charlster": "Gagnon54",
	"moraxam":   "Morax",
	"yothos":    "yothos",
	"Choufleur": "Steoh",
	"g4m3f4c3":  "g4m3f4c3",
	"Oliiviier": "K.",
}

// runMessageSender starts a background routine that, every 30 seconds,
// fetches up to 10 oldest unsent clan messages and posts them to a channel named "testing-ground".
// After successful send, the messages are marked as sent in the database.
func (b *Bot) runMessageSender(ctx context.Context, channelName string, donationChannel string) {
	interval := 30 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if channelName == "" {
		channelName = defaultPendingChannel
	}

	if donationChannel == "" {
		donationChannel = defaultDonationChannel
	}

	// send immediately once on startup
	b.sendPendingMessages(channelName)

	for {
		select {
		case <-ctx.Done():
			log.Println("[messagesender] stopping message sender")
			return
		case <-ticker.C:
			b.sendPendingMessages(channelName)
		}
	}
}

// sendPendingMessages fetches messages from DB and sends them to the testing-ground channel.
func (b *Bot) sendPendingMessages(channelName string) {
	if b.db == nil {
		log.Println("[messagesender] no db available")
		return
	}

	msgs, err := model.GetMessages(b.db)
	if err != nil {
		log.Printf("[messagesender] failed to get messages: %v", err)
		return
	}
	if len(msgs) == 0 {
		return
	}

	// find channel ID for name "testing-ground" across guilds the bot is in
	channelID := b.findChannelIDByName(channelName)
	if channelID == "" {
		log.Printf("[messagesender] channel %v not found", channelName)
		return
	}

	sentIDs := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		text := formatMessage(m)
		if _, err := b.session.ChannelMessageSend(channelID, text); err != nil {
			log.Printf("[messagesender] failed to send message id=%d: %v", m.ID, err)
			// don't mark as sent; continue to next
			continue
		}
		sentIDs = append(sentIDs, m.ID)

		// Check if this was a large gold donation and send celebration message
		b.checkForLargeGoldDonation(m)

		// small pause to avoid hitting rate limits
		time.Sleep(150 * time.Millisecond)
	}

	if len(sentIDs) > 0 {
		if err := model.MarkMessagesSent(b.db, sentIDs); err != nil {
			log.Printf("[messagesender] failed to mark messages sent: %v", err)
		}
	}
}

// checkForLargeGoldDonation checks if a message is a gold donation > 1 million.
// If so, sends an additional celebration message to the #general channel.
func (b *Bot) checkForLargeGoldDonation(msg model.ClanMessage) {
	// Pattern: "playername added NNNNNNx Gold."
	re := regexp.MustCompile(`^(.+?)\s+added\s+(\d+)x\s+Gold\.$`)
	matches := re.FindStringSubmatch(msg.Message)

	if len(matches) != 3 {
		return
	}

	playerName := matches[1]
	amountStr := matches[2]

	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return
	}

	// Only celebrate donations >= 1 million
	if amount < 1000000 {
		return
	}

	// Find the #general channel
	generalChannelID := b.findChannelIDByName("general")
	if generalChannelID == "" {
		log.Println("[messagesender] general channel not found, cannot send celebration message")
		return
	}

	// Convert UTC timestamp to EST/EDT for the embed
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("[messagesender] failed to load EST timezone: %v", err)
		est = time.UTC // fallback to UTC
	}
	estTime := msg.Timestamp.In(est)
	discordMention, ok := memberToDiscord[playerName]
	if ok {
		playerName = "@" + discordMention
	}

	// Create celebration embed
	embed := &discordgo.MessageEmbed{
		Title:       "🔔🎉 Leadership Commendation",
		Description: "Leadership commends **" + playerName + "** for their exceptional Clan Vault contribution. This selfless act of organizational commitment exemplifies KlutzCo values. Well done.",
		Color:       0xFFD700, // Gold color
		Footer: &discordgo.MessageEmbedFooter{
			Text: estTime.Format("Jan _2, 2006 at 3:04 PM MST"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Amount Donated",
				Value:  formatAmount(amount) + " Gold",
				Inline: true,
			},
		},
	}

	if _, err := b.session.ChannelMessageSendEmbed(generalChannelID, embed); err != nil {
		log.Printf("[messagesender] failed to send celebration message for %s: %v", playerName, err)
	} else {
		log.Printf("[messagesender] sent celebration message for %s's %d gold donation", playerName, amount)
	}
}

func formatMessage(m model.ClanMessage) string {
	// Convert UTC timestamp to EST/EDT
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("[messagesender] failed to load EST timezone: %v", err)
		// fallback to UTC if timezone loading fails
		return "`[" + m.Timestamp.Format("Jan _2 15:04") + "]` " + m.Message
	}
	estTime := m.Timestamp.In(est)
	return "`[" + estTime.Format("Jan _2 15:04") + "]` " + m.Message
}

// formatAmount formats a number with comma separators (e.g., 1000000 -> "1,000,000")
func formatAmount(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}

	// Insert commas from right to left
	var result []byte
	for i, digit := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(digit))
	}
	return string(result)
}

// findChannelIDByName searches the bot's guilds for a text channel with the given name.
// Returns the first matching channel ID or empty string if not found.
func (b *Bot) findChannelIDByName(name string) string {
	// Prefer cached guilds from state
	if b.session == nil || b.session.State == nil {
		log.Println("[messagesender] session or state is nil")
		return ""
	}

	for _, g := range b.session.State.Guilds {
		channels, err := b.session.GuildChannels(g.ID)
		if err != nil {
			log.Printf("[messagesender] failed to list channels for guild %s: %v", g.ID, err)
			continue
		}
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildText && ch.Name == name {
				return ch.ID
			}
		}
	}
	return ""
}

func CanBotSend(s *discordgo.Session, channelID string) (bool, error) {
	botID := s.State.User.ID

	_, err := s.Channel(channelID)

	if err != nil {

		return false, err
	}

	perms, err := s.State.UserChannelPermissions(botID, channelID)
	if err != nil {
		return false, err
	}

	canView := perms&discordgo.PermissionViewChannel != 0
	canSend := perms&discordgo.PermissionSendMessages != 0

	return canView && canSend, nil
}
