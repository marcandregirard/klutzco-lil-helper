package bot

import (
	"context"
	"log"
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
		// small pause to avoid hitting rate limits
		time.Sleep(150 * time.Millisecond)
	}

	if len(sentIDs) > 0 {
		if err := model.MarkMessagesSent(b.db, sentIDs); err != nil {
			log.Printf("[messagesender] failed to mark messages sent: %v", err)
		}
	}
}

func formatMessage(m model.ClanMessage) string {
	return "[" + m.Timestamp.Format(time.RFC3339) + "] " + m.Message
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
