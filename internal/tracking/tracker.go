package tracking

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Tracker manages token savings tracking in SQLite.
type Tracker struct {
	db *sql.DB
}

// NewTracker opens or creates a SQLite database for tracking.
func NewTracker(dbPath string) (*Tracker, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &Tracker{db: db}, nil
}

// Track records a filtered command execution.
func (t *Tracker) Track(originalCmd, snipCmd string, inputTokens, outputTokens int, execTimeMs int64) error {
	saved := inputTokens - outputTokens
	pct := 0.0
	if inputTokens > 0 {
		pct = float64(saved) / float64(inputTokens) * 100
	}

	if _, err := t.db.Exec(insertSQL, originalCmd, snipCmd, inputTokens, outputTokens, saved, pct, execTimeMs); err != nil {
		return fmt.Errorf("track: %w", err)
	}

	// Cleanup old records
	t.db.Exec(cleanupSQL)

	return nil
}

// TrackPassthrough records a passthrough (unfiltered) command.
func (t *Tracker) TrackPassthrough(cmd string, tokens int, execTimeMs int64) error {
	return t.Track(cmd, cmd, tokens, tokens, execTimeMs)
}

// GetSummary returns aggregate tracking stats.
func (t *Tracker) GetSummary() (*Summary, error) {
	var s Summary
	err := t.db.QueryRow(summarySQL).Scan(&s.TotalCommands, &s.TotalSaved, &s.AvgSavings, &s.TotalTimeMs)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	return &s, nil
}

// GetDaily returns daily stats for the last N days.
func (t *Tracker) GetDaily(days int) ([]DayStats, error) {
	if days <= 0 {
		days = 7
	}
	rows, err := t.db.Query(dailySQL, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("daily: %w", err)
	}
	defer rows.Close()

	var stats []DayStats
	for rows.Next() {
		var d DayStats
		if err := rows.Scan(&d.Day, &d.Commands, &d.InputTokens, &d.OutputTokens, &d.SavedTokens, &d.AvgSavings); err != nil {
			return nil, fmt.Errorf("daily scan: %w", err)
		}
		stats = append(stats, d)
	}
	return stats, rows.Err()
}

// GetRecent returns the last N tracked commands.
func (t *Tracker) GetRecent(n int) ([]CommandRecord, error) {
	rows, err := t.db.Query(recentSQL, n)
	if err != nil {
		return nil, fmt.Errorf("recent: %w", err)
	}
	defer rows.Close()

	var records []CommandRecord
	for rows.Next() {
		var r CommandRecord
		if err := rows.Scan(&r.OriginalCmd, &r.SnipCmd, &r.InputTokens, &r.OutputTokens, &r.SavedTokens, &r.SavingsPct, &r.ExecTimeMs, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("recent scan: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// Close closes the database connection.
func (t *Tracker) Close() error {
	return t.db.Close()
}

// DBPath resolves the tracking database path.
func DBPath(configPath string) string {
	if p := os.Getenv("SNIP_DB_PATH"); p != "" {
		return p
	}
	if configPath != "" {
		return configPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "snip", "tracking.db")
}
