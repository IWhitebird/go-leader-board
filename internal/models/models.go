package models

import (
	"strconv"
	"strings"
	"time"
)

// HealthResponse is the response for the health endpoint
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// Score represents a player's score in a game
type Score struct {
	GameID    int64     `json:"game_id"`
	UserID    int64     `json:"user_id"`
	Score     uint64    `json:"score"`
	Timestamp time.Time `json:"timestamp"`
}

func ScoreCompare(a, b Score) int {
	if a.Score != b.Score {
		if a.Score > b.Score {
			return -1
		}
		return 1
	}
	if a.Timestamp != b.Timestamp {
		if a.Timestamp.Before(b.Timestamp) {
			return -1
		}
		return 1
	}
	return 0
}

// LeaderboardEntry represents a player's position on the leaderboard
type LeaderboardEntry struct {
	UserID int64  `json:"user_id"`
	Score  uint64 `json:"score"`
	Rank   uint64 `json:"rank"`
}

// TopLeadersResponse is the response for the top leaders endpoint
type TopLeadersResponse struct {
	GameID       int64              `json:"game_id"`
	Leaders      []LeaderboardEntry `json:"leaders"`
	TotalPlayers uint64             `json:"total_players"`
	Window       string             `json:"window,omitempty"`
}

// PlayerRankResponse is the response for the player rank endpoint
type PlayerRankResponse struct {
	GameID       int64   `json:"game_id"`
	UserID       int64   `json:"user_id"`
	Score        uint64  `json:"score"`
	Rank         uint64  `json:"rank"`
	Percentile   float64 `json:"percentile"`
	TotalPlayers uint64  `json:"total_players"`
	Window       string  `json:"window,omitempty"`
}

// TimeWindow represents the time period for leaderboard queries
type TimeWindow struct {
	// Duration in hours (0 = all time)
	Hours int
	// Display name of the window (e.g., "24h", "3d", "7d", "all")
	Display string
}

// GetLeaderboardIndex returns the array index for this time window
// This enables O(1) leaderboard access instead of map lookups
func (w TimeWindow) GetLeaderboardIndex() int {
	switch w.Hours {
	case 0:
		return 0 // AllTime
	case 24:
		return 1 // Last24Hours
	case 72:
		return 2 // Last3Days
	case 168:
		return 3 // Last7Days
	default:
		return 0 // Default to AllTime for unsupported windows
	}
}

// LeaderboardIndexCount represents the total number of predefined leaderboard types
const LeaderboardIndexCount = 4

// Predefined time windows with their indices
var (
	AllTime     = TimeWindow{Hours: 0, Display: "all"}
	Last24Hours = TimeWindow{Hours: 24, Display: "24h"}
	Last3Days   = TimeWindow{Hours: 72, Display: "3d"}
	Last7Days   = TimeWindow{Hours: 168, Display: "7d"}
)

// AllTimeWindows returns all predefined time windows in index order
func AllTimeWindows() [LeaderboardIndexCount]TimeWindow {
	return [LeaderboardIndexCount]TimeWindow{
		AllTime,     // index 0
		Last24Hours, // index 1
		Last3Days,   // index 2
		Last7Days,   // index 3
	}
}

func FromQueryParam(window string) TimeWindow {
	if window == "" {
		return AllTime
	}

	// Handle day-based windows
	if strings.HasSuffix(window, "d") {
		days, err := strconv.Atoi(window[:len(window)-1])
		if err == nil && days > 0 {
			return TimeWindow{
				Hours:   days * 24,
				Display: window,
			}
		}
	}

	// Handle hour-based windows
	if strings.HasSuffix(window, "h") {
		hours, err := strconv.Atoi(window[:len(window)-1])
		if err == nil && hours > 0 {
			return TimeWindow{
				Hours:   hours,
				Display: window,
			}
		}
	}

	// // Default to all time if parameter is not recognized
	return AllTime
}

// GetCutoffTime returns the cutoff time for filtering scores based on the time window
func (w TimeWindow) GetCutoffTime() *time.Time {
	if w.Hours <= 0 {
		return nil
	}

	cutoff := time.Now().UTC().Add(-time.Duration(w.Hours) * time.Hour)
	return &cutoff
}

// GetTimeRange returns start and end times for the window
func (w TimeWindow) GetTimeRange() (start *time.Time, end time.Time) {
	end = time.Now().UTC()

	if w.Hours <= 0 {
		return nil, end
	}

	startTime := end.Add(-time.Duration(w.Hours) * time.Hour)
	return &startTime, end
}

// String returns a string representation of the time window
func (w TimeWindow) String() string {
	if w.Hours <= 0 {
		return "all time"
	}
	return w.Display
}
