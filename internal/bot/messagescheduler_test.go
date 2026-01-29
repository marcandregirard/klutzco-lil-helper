package bot

import (
	"strings"
	"testing"
)

func TestBuildBossMessage(t *testing.T) {
	tests := []struct {
		name          string
		weekly        bool
		wantContent   []string
		wantReactions []string
	}{
		{
			name:   "Daily message",
			weekly: false,
			wantContent: []string{
				"What are your **daily boss quests today",
				":chicken:  Griffin",
				":imp:  Hades",
				":japanese_ogre:  Devil",
				":zap:  Zeus",
				":lion_face:  Chimera",
				":snake:  Medusa",
			},
			wantReactions: []string{"🐔", "😈", "👹", "⚡", "🦁", "🐍"},
		},
		{
			name:   "Weekly message",
			weekly: true,
			wantContent: []string{
				"What are your **weekly boss quests this week",
				":chicken:  Griffin",
				":imp:  Hades",
				":japanese_ogre:  Devil",
				":zap:  Zeus",
				":lion_face:  Chimera",
				":snake:  Medusa",
				":key:  Gem quest",
			},
			wantReactions: []string{"🐔", "😈", "👹", "⚡", "🦁", "🐍", "🔑"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, reactions := buildBossMessage(tt.weekly)

			for _, part := range tt.wantContent {
				if !strings.Contains(content, part) {
					t.Errorf("buildBossMessage() content missing %q, got %q", part, content)
				}
			}

			if len(reactions) != len(tt.wantReactions) {
				t.Fatalf("buildBossMessage() reactions length = %d, want %d", len(reactions), len(tt.wantReactions))
			}

			for i, r := range reactions {
				if r != tt.wantReactions[i] {
					t.Errorf("buildBossMessage() reaction[%d] = %q, want %q", i, r, tt.wantReactions[i])
				}
			}
		})
	}
}
