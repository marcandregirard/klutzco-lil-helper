package bot

import (
	"context"
	"log"
	"time"

	"klutco-lil-helper/internal/bosssummary"
)

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
	if summaryChannelID == "" {
		log.Printf("[bosssummary] channel not found: summary=%q", summaryChannelName)
		return nil
	}

	return bosssummary.RegenerateSummary(b.session, b.db, summaryChannelID)
}
