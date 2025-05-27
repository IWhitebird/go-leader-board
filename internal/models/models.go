package models

import (
	"time"

	"github.com/IWhitebird/go-leader-board/internal/logging"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

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

type LeaderboardEntry struct {
	UserID int64  `json:"user_id"`
	Score  uint64 `json:"score"`
	Rank   uint64 `json:"rank"`
}

type TopLeadersResponse struct {
	GameID       int64              `json:"game_id"`
	Leaders      []LeaderboardEntry `json:"leaders"`
	TotalPlayers uint64             `json:"total_players"`
	Window       string             `json:"window,omitempty"`
}

type PlayerRankResponse struct {
	GameID       int64   `json:"game_id"`
	UserID       int64   `json:"user_id"`
	Score        uint64  `json:"score"`
	Rank         uint64  `json:"rank"`
	Percentile   float64 `json:"percentile"`
	TotalPlayers uint64  `json:"total_players"`
	Window       string  `json:"window,omitempty"`
}

type TimeWindow struct {
	Hours   int
	Display string
}

func (w TimeWindow) GetLeaderboardIndex() int {
	switch w.Hours {
	case 0:
		return 0
	case 24:
		return 1
	case 72:
		return 2
	case 168:
		return 3
	default:
		return 0
	}
}

const LeaderboardIndexCount = 4

var (
	AllTime     = TimeWindow{Hours: 0, Display: "all"}
	Last24Hours = TimeWindow{Hours: 24, Display: "24h"}
	Last3Days   = TimeWindow{Hours: 72, Display: "3d"}
	Last7Days   = TimeWindow{Hours: 168, Display: "7d"}
)

func AllTimeWindows() [LeaderboardIndexCount]TimeWindow {
	return [LeaderboardIndexCount]TimeWindow{
		AllTime,
		Last24Hours,
		Last3Days,
		Last7Days,
	}
}

func FromQueryParam(window string) (TimeWindow, error) {
	switch window {
	case "":
		return AllTime, nil
	case "24h":
		return Last24Hours, nil
	case "3d":
		return Last3Days, nil
	case "7d":
		return Last7Days, nil
	default:
		logging.Error("invalid window", "window", window)
		return AllTime, nil
	}

	// // Handle day-based windows
	// if strings.HasSuffix(window, "d") {
	// 	days, err := strconv.Atoi(window[:len(window)-1])
	// 	if err == nil && days > 0 {
	// 		return TimeWindow{
	// 			Hours:   days * 24,
	// 			Display: window,
	// 		}
	// 	}
	// }

	// // Handle hour-based windows
	// if strings.HasSuffix(window, "h") {
	// 	hours, err := strconv.Atoi(window[:len(window)-1])
	// 	if err == nil && hours > 0 {
	// 		return TimeWindow{
	// 			Hours:   hours,
	// 			Display: window,
	// 		}
	// 	}
	// }

	// return AllTime
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
