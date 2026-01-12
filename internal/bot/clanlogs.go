package bot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"klutco-lil-helper/internal/model"
)

// default values
const (
	defaultClanLogURL      = "https://query.idleclans.com/api/Clan/logs/clan/KlutzCo?limit=500"
	defaultClanLogInterval = 24 * time.Hour
)

// runClanLogFetcher runs on startup and then once every `interval` until ctx is canceled.
func (b *Bot) runClanLogFetcher(ctx context.Context, interval time.Duration, url string) {
	if url == "" {
		url = defaultClanLogURL
	}
	if interval <= 0 {
		interval = defaultClanLogInterval
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// immediate fetch
	if err := fetchAndStoreClanLogs(ctx, client, url, b.db); err != nil {
		log.Printf("[clanlogs] initial fetch failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[clanlogs] stopping clan log fetcher")
			return
		case <-ticker.C:
			if err := fetchAndStoreClanLogs(ctx, client, url, b.db); err != nil {
				log.Printf("[clanlogs] scheduled fetch failed: %v", err)
			}
		}
	}
}

// fetchAndStoreClanLogs performs a single fetch + parse + store operation.
func fetchAndStoreClanLogs(ctx context.Context, client *http.Client, url string, db *sql.DB) error {
	if url == "" {
		return errors.New("empty url")
	}

	var lastErr error
	attempts := 3
	backoff := time.Second

	for i := 0; i < attempts; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[clanlogs] fetch attempt %d failed: %v", i+1, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
			continue
		}

		// ensure body closed
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = errors.New("non-2xx status: " + resp.Status)
			log.Printf("[clanlogs] fetch attempt %d returned status %s", i+1, resp.Status)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
			continue
		}

		msgs, err := parseRawClanMessages(body)
		if err != nil {
			return err
		}

		inserted := 0
		for _, m := range msgs {
			if err := model.InsertClanMessage(db, m); err != nil {
				log.Printf("[clanlogs] failed to insert message: %v", err)
				// continue on individual DB errors
				continue
			}
			inserted++
		}

		log.Printf("[clanlogs] fetched %d messages, attempted inserts: %d", len(msgs), inserted)
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("fetch failed")
}

// parseRawClanMessages decodes the API JSON into []model.ClanMessage.
// The API is expected to return a JSON array of objects with keys that map to ClanMessage fields.
func parseRawClanMessages(body []byte) ([]model.ClanMessage, error) {
	// Try unmarshalling into a generic slice of maps to be tolerant of field names
	var raw []map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var results []model.ClanMessage
	for _, item := range raw {
		var m model.ClanMessage

		// clan name
		if v, ok := item["clanName"]; ok {
			if s, ok := v.(string); ok {
				m.ClanName = s
			}
		} else if v, ok := item["clan_name"]; ok {
			if s, ok := v.(string); ok {
				m.ClanName = s
			}
		}

		// member username
		if v, ok := item["memberUsername"]; ok {
			if s, ok := v.(string); ok {
				m.MemberUsername = s
			}
		} else if v, ok := item["member_username"]; ok {
			if s, ok := v.(string); ok {
				m.MemberUsername = s
			}
		}

		// message
		if v, ok := item["message"]; ok {
			if s, ok := v.(string); ok {
				m.Message = s
			}
		}

		// timestamp - robust parsing
		if v, ok := item["timestamp"]; ok {
			if t, err := parseTimestamp(v); err == nil {
				m.Timestamp = t
			} else {
				log.Printf("[clanlogs] warning: failed to parse timestamp for item: %v", err)
				continue // skip this item
			}
		} else if v, ok := item["time"]; ok {
			if t, err := parseTimestamp(v); err == nil {
				m.Timestamp = t
			} else {
				log.Printf("[clanlogs] warning: failed to parse timestamp for item: %v", err)
				continue
			}
		} else {
			// missing timestamp -> skip
			log.Printf("[clanlogs] warning: item missing timestamp, skipping")
			continue
		}

		results = append(results, m)
	}

	return results, nil
}

// parseTimestamp accepts several possible timestamp representations and returns time in UTC.
func parseTimestamp(v interface{}) (time.Time, error) {
	switch t := v.(type) {
	case string:
		// try RFC3339 first
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed.UTC(), nil
		}
		// try common format
		if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
			return parsed.UTC(), nil
		}
		// try numeric in string
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			// assume seconds if 10 digits, ms if 13 digits
			if len(t) == 13 {
				return time.Unix(0, i*int64(time.Millisecond)).UTC(), nil
			}
			return time.Unix(i, 0).UTC(), nil
		}
		return time.Time{}, errors.New("unsupported string timestamp format")
	case float64:
		// JSON numbers are float64; treat as epoch seconds or ms depending on magnitude
		if t > 1e12 {
			// milliseconds
			secs := int64(t) / 1000
			nanos := int64(t) % 1000 * int64(time.Millisecond)
			return time.Unix(secs, nanos).UTC(), nil
		}
		return time.Unix(int64(t), 0).UTC(), nil
	case int64:
		// epoch seconds
		return time.Unix(t, 0).UTC(), nil
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return time.Unix(i, 0).UTC(), nil
		}
		if f, err := t.Float64(); err == nil {
			if f > 1e12 {
				secs := int64(f) / 1000
				nanos := int64(f) % 1000 * int64(time.Millisecond)
				return time.Unix(secs, nanos).UTC(), nil
			}
			return time.Unix(int64(f), 0).UTC(), nil
		}
		return time.Time{}, errors.New("unsupported json.Number timestamp")
	default:
		return time.Time{}, errors.New("unsupported timestamp type")
	}
}
