package bot

import (
	"context"
	"log"
	"strings"
	"time"

	"klutco-lil-helper/internal/model"

	"github.com/bwmarrin/discordgo"
)

const defaultPendingChannel = "testing-ground"

// runMessageSender starts a background routine that, every 30 seconds,
// fetches up to 10 oldest unsent clan messages and posts them to a channel named "testing-ground".
// After successful send, the messages are marked as sent in the database.
func (b *Bot) runMessageSender(ctx context.Context, channelName string) {
	interval := 30 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if channelName == "" {
		channelName = defaultPendingChannel
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

// sendPendingMessages fetches messages from DB and sends them to their designated channels.
func (b *Bot) sendPendingMessages(defaultChannelName string) {
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

	// Group messages by channel based on content
	messagesByChannel := make(map[string][]model.ClanMessage)
	for _, m := range msgs {
		channelName := determineChannel(m, defaultChannelName)
		messagesByChannel[channelName] = append(messagesByChannel[channelName], m)
	}

	// Send messages to each channel
	sentIDs := make([]int64, 0, len(msgs))
	for channelName, channelMsgs := range messagesByChannel {
		// find channel ID for this channel name
		channelID := b.findChannelIDByName(channelName)
		if channelID == "" {
			log.Printf("[messagesender] channel %q not found, skipping %d messages", channelName, len(channelMsgs))
			continue
		}

		for _, m := range channelMsgs {
			text := formatMessage(m)
			if _, err := b.session.ChannelMessageSend(channelID, text); err != nil {
				log.Printf("[messagesender] failed to send message id=%d to channel %q: %v", m.ID, channelName, err)
				// don't mark as sent; continue to next
				continue
			}
			sentIDs = append(sentIDs, m.ID)
			// small pause to avoid hitting rate limits
			time.Sleep(150 * time.Millisecond)
		}
	}

	if len(sentIDs) > 0 {
		if err := model.MarkMessagesSent(b.db, sentIDs); err != nil {
			log.Printf("[messagesender] failed to mark messages sent: %v", err)
		}
	}
}

// determineChannel determines which Discord channel a message should be sent to
// based on its content.
// - Celebration messages (gold > 1M) go to "general"
// - Gold donation messages go to "corporate-oversight"
// - All other messages go to the default channel
func determineChannel(msg model.ClanMessage, defaultChannel string) string {
	// Celebration messages (from gold donations > 1M) go to general
	if strings.HasPrefix(msg.Message, "Leadership commends ") {
		return "general"
	}
	// Gold donation messages go to corporate-oversight
	if strings.Contains(msg.Message, "added ") && strings.Contains(msg.Message, "x Gold.") {
		return "corporate-oversight"
	}
	// All other messages go to the default channel
	return defaultChannel
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
