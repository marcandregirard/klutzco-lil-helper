package bot

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// runBossScheduler posts a boss-quest message every day at midnight UTC to the named channel.
// On Sundays it posts the weekly variant. It respects ctx cancellation.
func (b *Bot) runBossScheduler(ctx context.Context, channelName string) {
	// compute initial wait until next UTC midnight
	next := nextUTCMidnight(time.Now().UTC())
	wait := time.Until(next)
	log.Printf("[messagescheduler] boss scheduler will first run at %s (in %s)", next.Format(time.RFC3339), wait)

	// wait until first trigger or ctx cancellation
t1:
	select {
	case <-ctx.Done():
		log.Println("[messagescheduler] context cancelled before first run")
		return
	case <-time.After(wait):
		// continue to loop
	}

	for {
		// decide weekly vs daily based on UTC weekday
		now := time.Now().UTC()
		isWeekly := now.Weekday() == time.Sunday

		if err := b.postBossMessage(channelName, isWeekly); err != nil {
			log.Printf("[messagescheduler] failed to post boss message: %v", err)
		}

		// compute next run: next midnight UTC
		next = nextUTCMidnight(now.Add(1 * time.Minute))
		// sleep until next, but wake early if ctx cancelled
		wait = time.Until(next)
		select {
		case <-ctx.Done():
			log.Println("[messagescheduler] context cancelled, stopping scheduler")
			return
		case <-time.After(wait):
			// continue
		}
		// safety: loop continues
		goto t1
	}
}

// postBossMessage finds the channel by name, verifies permissions, sends the message, and adds reactions.
// If weekly is true, the message uses the word 'weekly' instead of 'daily'.
func (b *Bot) postBossMessage(channelName string, weekly bool) error {
	if b.session == nil || b.session.State == nil {
		return nil // session not ready; we'll try again next run
	}

	channelID := b.findChannelIDByName(channelName)
	if channelID == "" {
		log.Printf("[messagescheduler] channel %s not found", channelName)
		return nil
	}

	canSend, err := CanBotSend(b.session, channelID)
	if err != nil {
		log.Printf("[messagescheduler] permission check failed for channel %s: %v", channelName, err)
		// continue attempting to send; try once and observe API error
	}
	if !canSend {
		log.Printf("[messagescheduler] bot lacks view/send permissions for channel %s", channelName)
		return nil
	}

	content, reactions := buildBossMessage(weekly)

	// send message with retries
	var m *discordgo.Message
	for attempt := 1; attempt <= 3; attempt++ {
		msg, err := b.session.ChannelMessageSend(channelID, content)
		if err == nil {
			m = msg
			break
		}
		log.Printf("[messagescheduler] attempt %d: failed to send message to %s: %v", attempt, channelName, err)
		// backoff
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if m == nil {
		return nil
	}

	// add reactions sequentially (best-effort)
	for _, r := range reactions {
		if strings.HasPrefix(r, ":") {
			// custom emoji placeholder in form :name:id not expected; use raw as-is
		}
		for attempt := 1; attempt <= 3; attempt++ {
			if err := b.session.MessageReactionAdd(m.ChannelID, m.ID, r); err != nil {
				log.Printf("[messagescheduler] attempt %d: failed to add reaction %s: %v", attempt, r, err)
				// short backoff before retrying
				time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
				continue
			}
			break
		}
	}

	log.Printf("[messagescheduler] posted boss message (weekly=%v) to channel %s", weekly, channelName)
	return nil
}

// buildBossMessage returns the content and the list of unicode reactions to add.
func buildBossMessage(weekly bool) (string, []string) {
	word := "daily"
	if weekly {
		word = "weekly"
	}
	content := "**Who has " + word + " quests for what bosses today?**  :chicken:  Griffin :imp:  Hades :japanese_ogre:  Devil :zap:  Zeus :lion_face:  Chimera :snake:  Medusa"

	// Unicode emoji to react with (match visual order): chicken, imp, japanese_ogre, zap, lion_face, snake
	reactions := []string{"🐔", "😈", "👺", "⚡", "🦁", "🐍"}
	return content, reactions
}

// nextUTCMidnight returns the next occurrence of 00:00 UTC at or after the provided time.
func nextUTCMidnight(t time.Time) time.Time {
	y := t.Year()
	m := t.Month()
	d := t.Day()
	// if already at midnight exactly or after, move to next day
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	// move to next day at 00:00 UTC
	next := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	return next
}
