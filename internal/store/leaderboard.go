package store

import (
	"log"
	"sync"
	"time"

	cache "github.com/ringg-play/leaderboard-realtime/internal/cache"
	models "github.com/ringg-play/leaderboard-realtime/internal/models"
)

type LeaderBoard struct {
	mu         sync.RWMutex
	scoresList *cache.SkipList[int64, models.Score]
}

type GameLeaderboard struct {
	leaderboards [models.LeaderboardIndexCount]*LeaderBoard
}

func NewGameLeaderboard() *GameLeaderboard {
	gl := &GameLeaderboard{}

	// Initialize all leaderboards using array indices for O(1) access
	for i := range models.LeaderboardIndexCount {
		gl.leaderboards[i] = &LeaderBoard{
			scoresList: cache.NewSkipList[int64](models.ScoreCompare),
		}
	}
	return gl
}

func (gl *GameLeaderboard) getLeaderboard(window models.TimeWindow) *LeaderBoard {
	index := window.GetLeaderboardIndex()
	if index >= 0 && index < models.LeaderboardIndexCount {
		return gl.leaderboards[index]
	}
	log.Printf("Leaderboard index %d not found for window %v", index, window)
	return gl.leaderboards[0] // fallback to AllTime
}

// getCutoffTime returns the cutoff time for a given time window
func (gl *GameLeaderboard) getCutoffTime(window models.TimeWindow) time.Time {
	if window.Hours > 0 {
		return time.Now().UTC().Add(-time.Duration(window.Hours) * time.Hour)
	}
	return time.Time{} // AllTime has no cutoff
}

// isScoreValid checks if a score is valid for a given time window
func (gl *GameLeaderboard) isScoreValid(window models.TimeWindow, timestamp time.Time) bool {
	if window.Hours == 0 {
		return true
	}
	cutoff := gl.getCutoffTime(window)
	return timestamp.After(cutoff)
}

type LockType int

const (
	LockTypeRead LockType = iota
	LockTypeWrite
	LockTypeDirtyRead
)

// withLeaderboard executes a function with a leaderboard, handling locking
func (gl *GameLeaderboard) withLeaderboard(window models.TimeWindow, lockType LockType, fn func(*LeaderBoard)) {
	lb := gl.getLeaderboard(window)
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

	for _, window := range models.AllTimeWindows() {
		if !gl.isScoreValid(window, timestamp) {
			continue
		}

		gl.withLeaderboard(window, LockTypeWrite, func(lb *LeaderBoard) {
			// Use InsertOrUpdate to ensure user uniqueness with best score
			lb.scoresList.InsertOrUpdate(userID, newScore)
		})
	}
}

func (gl *GameLeaderboard) AddScoreBatch(scores []models.Score) {
	for _, score := range scores {
		gl.AddScore(score.UserID, score.Score, score.Timestamp)
	}

	// **Optimize this**
	// var validScores []models.Score
	// for _, score := range scores {
	// 	for _, window := range models.AllTimeWindows() {
	// 		if !gl.isScoreValid(window, score.Timestamp) {
	// 			continue
	// 		}
	// 		validScores = append(validScores, score)
	// 	}
	// }

	// for _, score := range validScores {
	// 	gl.AddScore(score.UserID, score.Score, score.Timestamp)
	// }
}

// GetTopK returns the top k entries from the leaderboard
func (gl *GameLeaderboard) GetTopK(k int, window models.TimeWindow) []models.LeaderboardEntry {
	var result []models.LeaderboardEntry

	gl.withLeaderboard(window, LockTypeDirtyRead, func(lb *LeaderBoard) {
		entries := lb.scoresList.GetTopK(k)
		result = make([]models.LeaderboardEntry, len(entries))

		for i, entry := range entries {
			result[i] = models.LeaderboardEntry{
				UserID: entry.Key,
				Score:  entry.Value.Score,
				Rank:   uint64(entry.Rank),
			}
		}
	})

	return result
}

// GetRankAndPercentile gets a player's rank and percentile in the leaderboard
func (gl *GameLeaderboard) GetRankAndPercentile(userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, bool) {
	var rank uint64
	var percentile float64
	var userScore uint64
	var total uint64
	var found bool

	gl.withLeaderboard(window, LockTypeDirtyRead, func(lb *LeaderBoard) {
		r, rankFound := lb.scoresList.GetRank(userID)
		if !rankFound {
			return
		}

		// Get the user's score
		scoreKey, scoreFound := lb.scoresList.Search(userID)
		if !scoreFound {
			return
		}

		rank = uint64(r)
		userScore = scoreKey.Score
		total = uint64(lb.scoresList.GetLength())
		percentile = 100.0 * float64(total-rank) / float64(total)
		found = true
	})

	return rank, percentile, userScore, total, found
}

// TotalPlayers returns the total number of players in the leaderboard
func (gl *GameLeaderboard) TotalPlayers(window models.TimeWindow) uint64 {
	var total uint64

	gl.withLeaderboard(window, LockTypeDirtyRead, func(lb *LeaderBoard) {
		total = uint64(lb.scoresList.GetLength())
	})

	return total
}

// CleanOldEntries removes old entries from the hour-based leaderboards
func (gl *GameLeaderboard) CleanOldEntries() {
	// for _, window := range models.AllTimeWindows() {
	// 	if window == models.AllTime {
	// 		continue // Skip all-time leaderboard
	// 	}

	// 	cutoffTime := gl.getCutoffTime(window)

	// 	gl.withLeaderboard(window, LockTypeWrite, func(lb *LeaderBoard) {
	// 		// Get all expired entries
	// 		expiredEntries := lb.scoresList.GetAllExpiredEntries(func(score models.Score) bool {
	// 			return score.Timestamp.Before(cutoffTime)
	// 		})

	// 		// Delete expired entries
	// 		for _, entry := range expiredEntries {
	// 			lb.scoresList.Delete(entry.Key)
	// 		}

	// 		log.Printf("Cleaned %d expired entries from %s leaderboard", len(expiredEntries), window.Display)
	// 	})
	// }
}
