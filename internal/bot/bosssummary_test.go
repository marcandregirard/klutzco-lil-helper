package bot

import (
	"testing"
	"time"
)

func TestNextEasternTime(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		now        time.Time
		hour       int
		minute     int
		wantDay    int
		wantHour   int
		wantMinute int
	}{
		{
			name:       "before 9:30 AM Eastern, same day",
			now:        time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC), // 9:00 AM EST
			hour:       9,
			minute:     30,
			wantDay:    15,
			wantHour:   9,
			wantMinute: 30,
		},
		{
			name:       "after 9:30 AM Eastern, next day",
			now:        time.Date(2025, 1, 15, 15, 0, 0, 0, time.UTC), // 10:00 AM EST
			hour:       9,
			minute:     30,
			wantDay:    16,
			wantHour:   9,
			wantMinute: 30,
		},
		{
			name:       "exactly 9:30 AM Eastern, next day",
			now:        time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC), // 9:30 AM EST
			hour:       9,
			minute:     30,
			wantDay:    16,
			wantHour:   9,
			wantMinute: 30,
		},
		{
			name:       "custom time 11:00 AM",
			now:        time.Date(2025, 1, 15, 15, 0, 0, 0, time.UTC), // 10:00 AM EST
			hour:       11,
			minute:     0,
			wantDay:    15,
			wantHour:   11,
			wantMinute: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextEasternTime(tt.now, tt.hour, tt.minute)
			eastern := got.In(loc)
			if eastern.Hour() != tt.wantHour {
				t.Errorf("hour = %d, want %d", eastern.Hour(), tt.wantHour)
			}
			if eastern.Minute() != tt.wantMinute {
				t.Errorf("minute = %d, want %d", eastern.Minute(), tt.wantMinute)
			}
			if eastern.Day() != tt.wantDay {
				t.Errorf("day = %d, want %d", eastern.Day(), tt.wantDay)
			}
		})
	}
}
