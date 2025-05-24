package store

import (
	"log"
	"sync"
	"time"

	cache "github.com/ringg-play/leaderboard-realtime/internal/cache"
	models "github.com/ringg-play/leaderboard-realtime/internal/models"
)

type GameLeaderboardType int

const (
	AllTime GameLeaderboardType = iota
	Last24Hours
	Last3Days
	Last7Days
)

// GlToTime maps leaderboard types to their hour values
var GlToTime = map[GameLeaderboardType]int{
	AllTime:     0,
	Last24Hours: 24,
	Last3Days:   72,
	Last7Days:   168,
}

var TimeToGl = map[int]GameLeaderboardType{
	0:   AllTime,
	24:  Last24Hours,
	72:  Last3Days,
	168: Last7Days,
}

type LeaderBoard struct {
	mu         sync.RWMutex
	userScores map[int64]models.Score
	scoresList *cache.SkipList[models.Score, int64]
}

type GameLeaderboard struct {
	leaderboards map[GameLeaderboardType]*LeaderBoard
}

// NewGameLeaderboard creates a new game leaderboard
func NewGameLeaderboard() *GameLeaderboard {
	gl := &GameLeaderboard{
		leaderboards: make(map[GameLeaderboardType]*LeaderBoard),
	}

	for boardType := range GlToTime {
		gl.leaderboards[boardType] = &LeaderBoard{
			scoresList: cache.NewSkipList[models.Score, int64](models.ScoreCompare),
			userScores: make(map[int64]models.Score),
		}
	}
	return gl
}

func (gl *GameLeaderboard) getLeaderboard(boardType GameLeaderboardType) *LeaderBoard {
	if lb, exists := gl.leaderboards[boardType]; exists {
		return lb
	}
	log.Printf("Leaderboard %v not found", boardType)
	return nil
}

// getCutoffTime returns the cutoff time for a given leaderboard type
func (gl *GameLeaderboard) getCutoffTime(boardType GameLeaderboardType) time.Time {
	if hours, exists := GlToTime[boardType]; exists && hours > 0 {
		return time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	}
	return time.Time{} // AllTime has no cutoff
}

// isScoreValid checks if a score is valid for a given leaderboard type
func (gl *GameLeaderboard) isScoreValid(boardType GameLeaderboardType, timestamp time.Time) bool {
	if boardType == AllTime {
		return true
	}
	cutoff := gl.getCutoffTime(boardType)
	return timestamp.After(cutoff)
}

type LockType int

const (
	LockTypeRead LockType = iota
	LockTypeWrite
	LockTypeDirtyRead
)

// withLeaderboard executes a function with a leaderboard, handling locking
func (gl *GameLeaderboard) withLeaderboard(boardType GameLeaderboardType, lockType LockType, fn func(*LeaderBoard)) {
	lb := gl.getLeaderboard(boardType)
	if lb == nil {
		return
	}

	switch lockType {
	case LockTypeRead:
		// Skip locking for read operations to allow dirty reads
	case LockTypeWrite:
		lb.mu.Lock()
		defer lb.mu.Unlock()
	case LockTypeDirtyRead:
		lb.mu.Lock()
		defer lb.mu.Unlock()
	}
	fn(lb)
}

// AddScore adds a new score to the leaderboard
func (gl *GameLeaderboard) AddScore(userID int64, score uint64, timestamp time.Time) {
	newScore := models.Score{
		UserID:    userID,
		Score:     score,
		Timestamp: timestamp,
	}

	for boardType := range gl.leaderboards {
		if !gl.isScoreValid(boardType, timestamp) {
			continue
		}

		gl.withLeaderboard(boardType, LockTypeWrite, func(lb *LeaderBoard) {
			if existing, exists := lb.userScores[userID]; exists && existing.Score >= score {
				return
			}

			if existing, exists := lb.userScores[userID]; exists {
				lb.scoresList.Delete(existing)
			}

			lb.scoresList.Insert(newScore, userID)
			lb.userScores[userID] = newScore
		})
	}
}

// GetTopK returns the top k entries from the leaderboard
func (gl *GameLeaderboard) GetTopK(k int, window models.TimeWindow) []models.LeaderboardEntry {
	boardType := TimeToGl[window.Hours]
	var result []models.LeaderboardEntry

	gl.withLeaderboard(boardType, LockTypeDirtyRead, func(lb *LeaderBoard) {
		entries := lb.scoresList.GetTopK(k)
		result = make([]models.LeaderboardEntry, len(entries))

		for i, entry := range entries {
			result[i] = models.LeaderboardEntry{
				UserID: entry.Value,
				Score:  entry.Key.Score,
				Rank:   uint64(entry.Rank),
			}
		}
	})

	return result
}

// GetRankAndPercentile gets a player's rank and percentile in the leaderboard
func (gl *GameLeaderboard) GetRankAndPercentile(userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, bool) {
	boardType := TimeToGl[window.Hours]
	var rank uint64
	var percentile float64
	var userScoreValue uint64
	var total uint64
	var found bool

	gl.withLeaderboard(boardType, LockTypeDirtyRead, func(lb *LeaderBoard) {
		userScore, exists := lb.userScores[userID]
		if !exists {
			return
		}

		r, rankFound := lb.scoresList.GetRank(userScore)
		if !rankFound {
			return
		}

		rank = uint64(r)
		total = uint64(lb.scoresList.GetLength())
		percentile = 100.0 * float64(total-rank) / float64(total)
		userScoreValue = userScore.Score
		found = true
	})

	return rank, percentile, userScoreValue, total, found
}

// TotalPlayers returns the total number of players in the leaderboard
func (gl *GameLeaderboard) TotalPlayers(window models.TimeWindow) uint64 {
	boardType := TimeToGl[window.Hours]
	var total uint64

	gl.withLeaderboard(boardType, LockTypeDirtyRead, func(lb *LeaderBoard) {
		total = uint64(lb.scoresList.GetLength())
	})

	return total
}

// CleanOldEntries removes entries older than the specified cutoff time
func (gl *GameLeaderboard) CleanOldEntries() {
	// Implementation for cleaning old entries
	// This would iterate through time-based leaderboards and remove expired entries
}
