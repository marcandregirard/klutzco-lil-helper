package bot

import (
	"reflect"
	"sort"
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
		{"350298028902711308", "K."},
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
			expectedNames: []string{"Bob (w)"},
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
			expectedNames: []string{"Alice", "Bob (w)"},
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
			sort.Strings(tt.expectedNames)
			if !reflect.DeepEqual(got, tt.expectedNames) {
				t.Errorf("got %v, want %v", got, tt.expectedNames)
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
