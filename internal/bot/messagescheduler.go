package bot

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// runBossScheduler posts a boss-quest message every day at midnight UTC to the named channel.
// On Sundays it posts the weekly variant. It respects ctx cancellation.
func (b *Bot) runBossScheduler(ctx context.Context, channelName string) {
	// compute initial next UTC midnight
	next := nextUTCMidnight(time.Now().UTC())

	for {
		wait := time.Until(next)
		log.Printf("[messagescheduler] boss scheduler will first run at %s (in %s)", next.Format(time.RFC3339), wait)
		// use a timer so we can stop it if context is cancelled
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			// stop timer to avoid goroutine leak
			if !timer.Stop() {
				// drained or expired; nothing to do
			}
			log.Println("[messagescheduler] context cancelled, stopping scheduler")
			return
		case <-timer.C:
			// time to post
		}

		// decide weekly vs daily based on UTC weekday
		now := time.Now().UTC()
		isWeekly := now.Weekday() == time.Monday

		// post the weekly message if applicable
		if isWeekly {
			if err := b.postBossMessage(channelName, true); err != nil {
				log.Printf("[messagescheduler] failed to post boss message: %v", err)
			}
		}

		// post the daily message
		if err := b.postBossMessage(channelName, false); err != nil {
			log.Printf("[messagescheduler] failed to post boss message: %v", err)
		}

		// schedule next run at the following midnight
		next = next.Add(24 * time.Hour)
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
	// For daily use 'today?', for weekly avoid 'today' at the end; use 'this week?'
	dateSuffix := ""

	if !weekly {
		now := time.Now().UTC()
		day := now.Day()
		// determine ordinal suffix
		suffix := "th"
		if day%100 < 11 || day%100 > 13 {
			switch day % 10 {
			case 1:
				suffix = "st"
			case 2:
				suffix = "nd"
			case 3:
				suffix = "rd"
			}
		}
		dateSuffix = " (" + now.Format("Jan ") + strconv.Itoa(day) + suffix + ")"
	}

	// removed leading spaces from ending strings to avoid double spaces when concatenated
	ending := "today" + dateSuffix
	if weekly {
		ending = "this week"
	}

	// include date like (Jan 20th) for daily messages
	content := "What are your **" + word + " boss quests " + ending + "?**\n\n  :chicken:  Griffin\n :imp:  Hades\n :japanese_ogre:  Devil\n :zap:  Zeus\n :lion_face:  Chimera\n :snake:  Medusa"

	// Unicode emoji to react with (match visual order): chicken, imp, japanese_ogre, zap, lion_face, snake
	reactions := []string{"🐔", "😈", "👹", "⚡", "🦁", "🐍"}

	if weekly {
		content += "\n :key:  Gem quest"
		reactions = append(reactions, "🔑")
	}
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
