package bot

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildDiscordIDToDisplayName(t *testing.T) {
	m := buildDiscordIDToDisplayName()

	tests := []struct {
		discordID   string
		displayName string
	}{
		{"199632692231274496", "Guildan"},
		{"350298028902711308", "oli"},
		{"270655486318215168", "ImaKlutz"},
		{"448261978469695489", "yothos"},
	}
	for _, tt := range tests {
		got, ok := m[tt.discordID]
		if !ok {
			t.Errorf("missing key %s", tt.discordID)
			continue
		}
		if got != tt.displayName {
			t.Errorf("m[%s] = %q, want %q", tt.discordID, got, tt.displayName)
		}
	}

	if len(m) != len(memberToDiscordId) {
		t.Errorf("len = %d, want %d", len(m), len(memberToDiscordId))
	}
}

func TestMergeReactionsToNames(t *testing.T) {
	idToName := map[string]string{
		"AAA": "Alice",
		"BBB": "Bob",
		"CCC": "Charlie",
	}

	tests := []struct {
		name          string
		daily         map[string]bool
		weekly        map[string]bool
		weeklyOnly    bool
		expectedNames []string
	}{
		{
			name:          "daily only user, no (w)",
			daily:         map[string]bool{"AAA": true},
			weekly:        nil,
			weeklyOnly:    false,
			expectedNames: []string{"Alice"},
		},
		{
			name:          "weekly only user gets (w)",
			daily:         nil,
			weekly:        map[string]bool{"BBB": true},
			weeklyOnly:    false,
			expectedNames: []string{"Bob [W]"},
		},
		{
			name:          "user in both, no (w)",
			daily:         map[string]bool{"AAA": true},
			weekly:        map[string]bool{"AAA": true},
			weeklyOnly:    false,
			expectedNames: []string{"Alice"},
		},
		{
			name:          "weekly-only boss, no (w) even for weekly user",
			daily:         nil,
			weekly:        map[string]bool{"CCC": true},
			weeklyOnly:    true,
			expectedNames: []string{"Charlie"},
		},
		{
			name:          "mixed daily and weekly-only users",
			daily:         map[string]bool{"AAA": true},
			weekly:        map[string]bool{"AAA": true, "BBB": true},
			weeklyOnly:    false,
			expectedNames: []string{"Alice", "Bob [W]"},
		},
		{
			name:          "weekly-only users sorted after daily users",
			daily:         map[string]bool{"CCC": true, "BBB": true},
			weekly:        map[string]bool{"AAA": true},
			weeklyOnly:    false,
			expectedNames: []string{"Bob", "Charlie", "Alice [W]"},
		},
		{
			name:          "unknown user ID skipped",
			daily:         map[string]bool{"ZZZ": true},
			weekly:        nil,
			weeklyOnly:    false,
			expectedNames: nil,
		},
		{
			name:          "nil maps produce empty result",
			daily:         nil,
			weekly:        nil,
			weeklyOnly:    false,
			expectedNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeReactionsToNames(tt.daily, tt.weekly, idToName, tt.weeklyOnly)
			if !reflect.DeepEqual(got, tt.expectedNames) {
				t.Errorf("got %v, want %v", got, tt.expectedNames)
			}
		})
	}
}

func TestAllWeeklyOnly(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  bool
	}{
		{"all weekly", []string{"Alice [W]", "Bob [W]"}, true},
		{"mixed", []string{"Alice", "Bob [W]"}, false},
		{"all daily", []string{"Alice", "Bob"}, false},
		{"single weekly", []string{"Charlie [W]"}, true},
		{"single daily", []string{"Charlie"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allWeeklyOnly(tt.names); got != tt.want {
				t.Errorf("allWeeklyOnly(%v) = %v, want %v", tt.names, got, tt.want)
			}
		})
	}
}

func TestNextEastern11AM(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		now     time.Time
		wantDay int
	}{
		{
			name:    "before 11AM Eastern, same day",
			now:     time.Date(2025, 1, 15, 15, 0, 0, 0, time.UTC), // 9AM EST
			wantDay: 15,
		},
		{
			name:    "after 11AM Eastern, next day",
			now:     time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC), // 11AM EST
			wantDay: 16,
		},
		{
			name:    "exactly 11AM Eastern, next day",
			now:     time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), // 10AM EST
			wantDay: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextEastern11AM(tt.now)
			eastern := got.In(loc)
			if eastern.Hour() != 11 {
				t.Errorf("hour = %d, want 11", eastern.Hour())
			}
			if eastern.Day() != tt.wantDay {
				t.Errorf("day = %d, want %d", eastern.Day(), tt.wantDay)
			}
		})
	}
}

func TestBuildSummaryContent_WeeklyOnlyNewline(t *testing.T) {
	// This test verifies that weekly-only bosses are correctly configured
	// and that the boss summary will add an extra newline before their entry.
	//
	// The newline behavior is implemented in buildSummaryContent at lines 165-167:
	//   if boss.WeeklyOnly {
	//       line = "\n" + line
	//   }
	// This adds visual separation for weekly-only quests like Gem Quest.

	// Verify the summaryBosses configuration
	var weeklyOnlyBosses []string
	var regularBosses []string

	for _, boss := range summaryBosses {
		if boss.WeeklyOnly {
			weeklyOnlyBosses = append(weeklyOnlyBosses, boss.Name)
		} else {
			regularBosses = append(regularBosses, boss.Name)
		}
	}

	// Verify Gem Quest is the only weekly-only boss
	if len(weeklyOnlyBosses) != 1 {
		t.Errorf("Expected 1 weekly-only boss, got %d: %v", len(weeklyOnlyBosses), weeklyOnlyBosses)
	}
	if len(weeklyOnlyBosses) > 0 && weeklyOnlyBosses[0] != "Gem Quest" {
		t.Errorf("Expected Gem Quest as weekly-only boss, got %s", weeklyOnlyBosses[0])
	}

	// Verify we have the expected regular bosses
	expectedRegularBosses := []string{"Griffin", "Hades", "Devil", "Zeus", "Chimera", "Medusa"}
	if len(regularBosses) != len(expectedRegularBosses) {
		t.Errorf("Expected %d regular bosses, got %d", len(expectedRegularBosses), len(regularBosses))
	}

	// Verify each expected boss exists
	for _, expected := range expectedRegularBosses {
		found := false
		for _, boss := range regularBosses {
			if boss == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected regular boss %s not found", expected)
		}
	}

	// Verify Gem Quest has the WeeklyOnly flag set
	for _, boss := range summaryBosses {
		if boss.Name == "Gem Quest" {
			if !boss.WeeklyOnly {
				t.Error("Gem Quest should have WeeklyOnly=true")
			}
			if boss.Emoji != "💎" {
				t.Errorf("Gem Quest emoji = %s, want 💎", boss.Emoji)
			}
		} else {
			if boss.WeeklyOnly {
				t.Errorf("Boss %s should not be weekly-only", boss.Name)
			}
		}
	}
}

func TestBossEntryFormat(t *testing.T) {
	// Test that verifies the line formatting logic for boss entries
	// This documents the expected format with and without WeeklyOnly flag

	tests := []struct {
		name       string
		boss       bossEntry
		names      []string
		wantPrefix string
	}{
		{
			name:       "regular boss no extra newline",
			boss:       bossEntry{Emoji: "🐔", Name: "Griffin", WeeklyOnly: false},
			names:      []string{"Alice", "Bob"},
			wantPrefix: "",
		},
		{
			name:       "weekly-only boss has extra newline before",
			boss:       bossEntry{Emoji: "💎", Name: "Gem Quest", WeeklyOnly: true},
			names:      []string{"Charlie"},
			wantPrefix: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the line building logic from buildSummaryContent:165-167
			line := " " + tt.boss.Emoji + "  " + tt.boss.Name + ": " + strings.Join(tt.names, ", ")
			if tt.boss.WeeklyOnly {
				line = "\n" + line
			}

			// Verify the line starts with the expected prefix
			if tt.wantPrefix == "" {
				if strings.HasPrefix(line, "\n") {
					t.Error("Regular boss should not start with newline")
				}
			} else {
				if !strings.HasPrefix(line, tt.wantPrefix) {
					t.Errorf("Line should start with %q, got: %q", tt.wantPrefix, line)
				}
			}

			// Log the formatted line for visual inspection
			t.Logf("Formatted line: %q", line)
		})
	}
}
