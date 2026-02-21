package tracking

const createTableSQL = `
CREATE TABLE IF NOT EXISTS commands (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME DEFAULT (datetime('now')),
	original_cmd TEXT NOT NULL,
	snip_cmd TEXT NOT NULL,
	input_tokens INTEGER NOT NULL,
	output_tokens INTEGER NOT NULL,
	saved_tokens INTEGER NOT NULL,
	savings_pct REAL NOT NULL,
	exec_time_ms INTEGER NOT NULL
);
`

const cleanupSQL = `DELETE FROM commands WHERE timestamp < datetime('now', '-90 days');`

const insertSQL = `
INSERT INTO commands (original_cmd, snip_cmd, input_tokens, output_tokens, saved_tokens, savings_pct, exec_time_ms)
VALUES (?, ?, ?, ?, ?, ?, ?);
`

const summarySQL = `
SELECT
	COUNT(*) as total_commands,
	COALESCE(SUM(saved_tokens), 0) as total_saved,
	COALESCE(SUM(saved_tokens) * 100.0 / NULLIF(SUM(input_tokens), 0), 0) as avg_savings,
	COALESCE(SUM(exec_time_ms), 0) as total_time_ms
FROM commands;
`

const dailySQL = `
SELECT
	date(timestamp) as day,
	COUNT(*) as commands,
	SUM(input_tokens) as input_tokens,
	SUM(output_tokens) as output_tokens,
	SUM(saved_tokens) as saved_tokens,
	COALESCE(SUM(saved_tokens) * 100.0 / NULLIF(SUM(input_tokens), 0), 0) as avg_savings
FROM commands
WHERE timestamp >= datetime('now', ? || ' days')
GROUP BY date(timestamp)
ORDER BY day DESC;
`

const recentSQL = `
SELECT original_cmd, snip_cmd, input_tokens, output_tokens, saved_tokens, savings_pct, exec_time_ms, timestamp
FROM commands
ORDER BY id DESC
LIMIT ?;
`

const byCommandSQL = `
SELECT
	original_cmd,
	COUNT(*) as count,
	SUM(input_tokens) as input_tokens,
	SUM(output_tokens) as output_tokens,
	SUM(saved_tokens) as saved_tokens,
	COALESCE(SUM(saved_tokens) * 100.0 / NULLIF(SUM(input_tokens), 0), 0) as avg_savings
FROM commands
GROUP BY original_cmd
ORDER BY saved_tokens DESC
LIMIT ?;
`

const weeklySQL = `
SELECT
	strftime('%Y-W%W', timestamp) as period,
	COUNT(*) as commands,
	SUM(input_tokens) as input_tokens,
	SUM(output_tokens) as output_tokens,
	SUM(saved_tokens) as saved_tokens,
	COALESCE(SUM(saved_tokens) * 100.0 / NULLIF(SUM(input_tokens), 0), 0) as avg_savings
FROM commands
WHERE timestamp >= datetime('now', ? || ' days')
GROUP BY strftime('%Y-W%W', timestamp)
ORDER BY period DESC;
`

const monthlySQL = `
SELECT
	strftime('%Y-%m', timestamp) as period,
	COUNT(*) as commands,
	SUM(input_tokens) as input_tokens,
	SUM(output_tokens) as output_tokens,
	SUM(saved_tokens) as saved_tokens,
	COALESCE(SUM(saved_tokens) * 100.0 / NULLIF(SUM(input_tokens), 0), 0) as avg_savings
FROM commands
WHERE timestamp >= datetime('now', ? || ' days')
GROUP BY strftime('%Y-%m', timestamp)
ORDER BY period DESC;
`

// Summary holds aggregate tracking stats.
type Summary struct {
	TotalCommands int
	TotalSaved    int
	AvgSavings    float64
	TotalTimeMs   int64
}

// DayStats holds daily tracking stats.
type DayStats struct {
	Day          string
	Commands     int
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	AvgSavings   float64
}

// CommandRecord holds a single tracked command.
type CommandRecord struct {
	OriginalCmd  string
	SnipCmd      string
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	SavingsPct   float64
	ExecTimeMs   int64
	Timestamp    string
}

// CommandStats holds aggregate stats per command.
type CommandStats struct {
	Command      string
	Count        int
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	AvgSavings   float64
}

// PeriodStats holds aggregate stats for a time period (week or month).
type PeriodStats struct {
	Period       string
	Commands     int
	InputTokens  int
	OutputTokens int
	SavedTokens  int
	AvgSavings   float64
}
