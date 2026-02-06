package bot

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"klutco-lil-helper/internal/model"

	"github.com/bwmarrin/discordgo"
)

// bossEntry defines a boss emoji and name for the summary output.
type bossEntry struct {
	Emoji      string
	Name       string
	WeeklyOnly bool // true for entries that only appear on the weekly poll
}

// summaryBosses is the ordered list of bosses matching the poll reactions.
var summaryBosses = []bossEntry{
	{"🐔", "Griffin", false},
	{"😈", "Hades", false},
	{"👹", "Devil", false},
	{"⚡", "Zeus", false},
	{"🦁", "Chimera", false},
	{"🐍", "Medusa", false},
	{"💎", "Gem Quest", true},
}

// buildDiscordIDToDisplayName creates a reverse map from Discord user ID
// to the display name used in summaries.
func buildDiscordIDToDisplayName() map[string]string {
	result := make(map[string]string, len(memberToDiscordId))
	for gameName, discordID := range memberToDiscordId {
		if displayName, ok := memberToDiscord[gameName]; ok {
			result[discordID] = displayName
		}
	}
	return result
}

// nextEasternTime returns the next occurrence of the specified time in America/New_York timezone.
func nextEasternTime(now time.Time, hour, minute int) time.Time {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("EST", -5*60*60)
	}

	eastern := now.In(loc)
	target := time.Date(eastern.Year(), eastern.Month(), eastern.Day(), hour, minute, 0, 0, loc)

	if !now.Before(target) {
		target = target.AddDate(0, 0, 1)
	}

	return target
}

// runBossSummary posts a boss fight summary every day at the specified time (Eastern).
func (b *Bot) runBossSummary(ctx context.Context, summaryChannel, bossChannel string, hour, minute int) {
	for {
		next := nextEasternTime(time.Now(), hour, minute)
		wait := time.Until(next)
		log.Printf("[bosssummary] next summary at %s (in %s)", next.Format(time.RFC3339), wait)

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				// drained or expired
			}
			log.Println("[bosssummary] context cancelled, stopping")
			return
		case <-timer.C:
			// time to post
		}

		if err := b.postBossSummary(summaryChannel, bossChannel); err != nil {
			log.Printf("[bosssummary] failed to post summary: %v", err)
		}
	}
}

// postBossSummary fetches reactions from the current daily/weekly polls and posts a summary.
func (b *Bot) postBossSummary(summaryChannelName, bossChannelName string) error {
	if b.session == nil || b.session.State == nil || b.db == nil {
		return nil
	}

	summaryChannelID := b.findChannelIDByName(summaryChannelName)
	bossChannelID := b.findChannelIDByName(bossChannelName)
	if summaryChannelID == "" || bossChannelID == "" {
		log.Printf("[bosssummary] channel not found: summary=%q boss=%q", summaryChannelName, bossChannelName)
		return nil
	}

	dailyMsgID, err := model.GetScheduledMessage(b.db, model.MessageTypeDaily, bossChannelID)
	if err != nil {
		return fmt.Errorf("get daily message: %w", err)
	}
	weeklyMsgID, err := model.GetScheduledMessage(b.db, model.MessageTypeWeekly, bossChannelID)
	if err != nil {
		return fmt.Errorf("get weekly message: %w", err)
	}

	idToName := buildDiscordIDToDisplayName()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("EST", -5*60*60)
	}
	isFriday := time.Now().In(loc).Weekday() == time.Friday

	content := buildSummaryContent(b.session, bossChannelID, dailyMsgID, weeklyMsgID, idToName, isFriday)

	m, err := b.session.ChannelMessageSend(summaryChannelID, content)
	if err != nil {
		return err
	}

	// Store the new message ID for future deletion
	if err := model.UpsertScheduledMessage(b.db, model.MessageTypeBossSummary, summaryChannelID, m.ID); err != nil {
		log.Printf("[bosssummary] failed to store summary message ID: %v", err)
	}

	return nil
}

// buildSummaryContent fetches reactions for each boss and builds the formatted message.
// When isFriday is false, bosses where every participant is weekly-only ([W]) are hidden.
func buildSummaryContent(
	s *discordgo.Session,
	bossChannelID, dailyMsgID, weeklyMsgID string,
	idToName map[string]string,
	isFriday bool,
) string {
	// Calculate max boss name length for alignment
	maxNameLen := 0
	for _, boss := range summaryBosses {
		if len(boss.Name) > maxNameLen {
			maxNameLen = len(boss.Name)
		}
	}

	var lines []string
	lines = append(lines, "Today's boss fight summaries:\n")

	for _, boss := range summaryBosses {
		var dailyUsers, weeklyUsers map[string]bool

		if !boss.WeeklyOnly {
			dailyUsers = fetchReactedUsers(s, bossChannelID, dailyMsgID, boss.Emoji)
			weeklyUsers = fetchReactedUsers(s, bossChannelID, weeklyMsgID, boss.Emoji)
		} else {
			weeklyUsers = fetchReactedUsers(s, bossChannelID, weeklyMsgID, boss.Emoji)
		}

		names := mergeReactionsToNames(dailyUsers, weeklyUsers, idToName, boss.WeeklyOnly)
		if len(names) == 0 {
			continue
		}

		// Hide bosses where every participant is weekly-only, unless it's Friday.
		if !boss.WeeklyOnly && !isFriday && allWeeklyOnly(names) {
			continue
		}

		// Pad boss name to align colons
		padding := strings.Repeat(" ", maxNameLen-len(boss.Name))
		line := " `" + boss.Emoji + "  " + boss.Name + padding + ":` " + strings.Join(names, " · ")
		if boss.WeeklyOnly {
			line = "\n" + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// allWeeklyOnly reports whether every name in the slice carries the "[W]" suffix.
func allWeeklyOnly(names []string) bool {
	for _, n := range names {
		if !strings.HasSuffix(n, "[W]") {
			return false
		}
	}
	return true
}

// fetchReactedUsers returns a set of non-bot user IDs that reacted with the given emoji.
func fetchReactedUsers(s *discordgo.Session, channelID, messageID, emoji string) map[string]bool {
	if messageID == "" {
		return nil
	}

	users, err := s.MessageReactions(channelID, messageID, emoji, 100, "", "")
	if err != nil {
		log.Printf("[bosssummary] failed to fetch reactions for %s on message %s: %v", emoji, messageID, err)
		return nil
	}

	result := make(map[string]bool, len(users))
	for _, u := range users {
		if u.Bot {
			continue
		}
		result[u.ID] = true
	}
	return result
}

// mergeReactionsToNames merges daily and weekly reaction sets into display names.
// Users who only appear in the weekly set get a "[W]" suffix,
// unless the boss is weekly-only (like Gem Quest).
func mergeReactionsToNames(
	dailyUsers, weeklyUsers map[string]bool,
	idToName map[string]string,
	weeklyOnlyBoss bool,
) []string {
	allUsers := make(map[string]bool)
	for id := range dailyUsers {
		allUsers[id] = true
	}
	for id := range weeklyUsers {
		allUsers[id] = true
	}

	var names []string
	for userID := range allUsers {
		displayName, ok := idToName[userID]
		if !ok {
			continue
		}

		inDaily := dailyUsers[userID]
		if !weeklyOnlyBoss && !inDaily {
			displayName += " [W]"
		}

		names = append(names, displayName)
	}

	sort.Slice(names, func(i, j int) bool {
		iw := strings.HasSuffix(names[i], "[W]")
		jw := strings.HasSuffix(names[j], "[W]")
		if iw != jw {
			return !iw // non-weekly first
		}
		return names[i] < names[j]
	})
	return names
}
