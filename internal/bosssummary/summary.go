package bosssummary

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"klutco-lil-helper/internal/model"

	"github.com/bwmarrin/discordgo"
)

// RegenerateSummary regenerates the boss summary in the given channel.
func RegenerateSummary(s *discordgo.Session, db *sql.DB, summaryChannelID string) error {
	if s == nil || db == nil {
		return fmt.Errorf("session or db is nil")
	}

	// Find the boss channel (where the daily/weekly polls are)
	var bossChannelID string
	if s.State != nil {
		for _, guild := range s.State.Guilds {
			for _, channel := range guild.Channels {
				if channel.Name == "tactical-dispatch" && channel.Type == discordgo.ChannelTypeGuildText {
					bossChannelID = channel.ID
					break
				}
			}
			if bossChannelID != "" {
				break
			}
		}
	}

	if bossChannelID == "" {
		return fmt.Errorf("boss channel not found")
	}

	// Get the daily and weekly poll message IDs
	dailyMsgID, err := model.GetScheduledMessage(db, model.MessageTypeDaily, bossChannelID)
	if err != nil {
		return fmt.Errorf("get daily message: %w", err)
	}
	weeklyMsgID, err := model.GetScheduledMessage(db, model.MessageTypeWeekly, bossChannelID)
	if err != nil {
		return fmt.Errorf("get weekly message: %w", err)
	}

	idToName := buildDiscordIDToDisplayName()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("EST", -5*60*60)
	}
	isFriday := time.Now().In(loc).Weekday() == time.Friday

	content := buildSummaryContent(s, bossChannelID, dailyMsgID, weeklyMsgID, idToName, isFriday)

	// Delete old summary message if it exists
	oldMsgID, _ := model.GetScheduledMessage(db, model.MessageTypeBossSummary, summaryChannelID)
	if oldMsgID != "" {
		_ = s.ChannelMessageDelete(summaryChannelID, oldMsgID)
	}

	// Post new summary
	m, err := s.ChannelMessageSend(summaryChannelID, content)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	// Store the new message ID
	if err := model.UpsertScheduledMessage(db, model.MessageTypeBossSummary, summaryChannelID, m.ID); err != nil {
		log.Printf("[bosssummary] failed to store summary message ID: %v", err)
	}

	return nil
}

// buildDiscordIDToDisplayName creates a reverse map from Discord user ID to display name.
func buildDiscordIDToDisplayName() map[string]string {
	result := make(map[string]string, len(model.MemberToDiscordID))
	for gameName, discordID := range model.MemberToDiscordID {
		if displayName, ok := model.MemberToDiscord[gameName]; ok {
			result[discordID] = displayName
		}
	}
	return result
}

// buildSummaryContent fetches reactions for each boss and builds the formatted message.
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
		line := boss.Emoji + "`  " + boss.Name + padding + ":` " + strings.Join(names, " · ")
		if boss.WeeklyOnly {
			line = "\n" + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
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

// allWeeklyOnly reports whether every name in the slice carries the "[W]" suffix.
func allWeeklyOnly(names []string) bool {
	for _, n := range names {
		if !strings.HasSuffix(n, "[W]") {
			return false
		}
	}
	return true
}

// bossEntry defines a boss emoji and name for the summary output.
type bossEntry struct {
	Emoji      string
	Name       string
	WeeklyOnly bool
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
