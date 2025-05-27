package store

import (
	"sync"
	"time"

	cache "github.com/IWhitebird/go-leader-board/internal/cache"
	"github.com/IWhitebird/go-leader-board/internal/logging"
	models "github.com/IWhitebird/go-leader-board/internal/models"
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
	logging.Error("Leaderboard index not found for window", window, "using AllTime fallback")
	return gl.leaderboards[0]
}

func (gl *GameLeaderboard) getCutoffTime(window models.TimeWindow) time.Time {
	if window.Hours > 0 {
		return time.Now().UTC().Add(-time.Duration(window.Hours) * time.Hour)
	}
	return time.Time{}
}

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

func (gl *GameLeaderboard) withLeaderboard(window models.TimeWindow, lockType LockType, fn func(*LeaderBoard)) {
	lb := gl.getLeaderboard(window)
	if lb == nil {
		return
	}

	switch lockType {
	case LockTypeRead:
	case LockTypeWrite:
		lb.mu.Lock()
		defer lb.mu.Unlock()
	case LockTypeDirtyRead:
		lb.mu.Lock()
		defer lb.mu.Unlock()
	}
	fn(lb)
}

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
			lb.scoresList.InsertOrUpdate(userID, newScore)
		})
	}
}

func (gl *GameLeaderboard) AddScoreBatch(scores []models.Score) {
	for _, score := range scores {
		gl.AddScore(score.UserID, score.Score, score.Timestamp)
	}
}

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

		scoreKey, scoreFound := lb.scoresList.Search(userID)
		if !scoreFound {
			return
		}

		rank = uint64(r)
		userScore = scoreKey.Score
		total = uint64(lb.scoresList.GetLength())
		percentile = 100.0 * float64(total-rank+1) / float64(total)
		found = true
	})

	return rank, percentile, userScore, total, found
}

func (gl *GameLeaderboard) TotalPlayers(window models.TimeWindow) uint64 {
	var total uint64

	gl.withLeaderboard(window, LockTypeDirtyRead, func(lb *LeaderBoard) {
		total = uint64(lb.scoresList.GetLength())
	})

	return total
}

func (gl *GameLeaderboard) CleanOldEntries() {
	for _, window := range models.AllTimeWindows() {
		cutoff := gl.getCutoffTime(window)
		gl.withLeaderboard(window, LockTypeWrite, func(lb *LeaderBoard) {
			toRemove := make([]int64, 0)

			entries := lb.scoresList.GetAll()
			for _, entry := range entries {
				if entry.Value.Timestamp.Before(cutoff) {
					toRemove = append(toRemove, entry.Key)
				}
			}

			for _, userID := range toRemove {
				lb.scoresList.Delete(userID)
			}
		})
	}
}
