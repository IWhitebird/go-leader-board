package db

import (
	"testing"
	"time"

	"github.com/ringg-play/leaderboard-realtime/models"
	"github.com/stretchr/testify/assert"
)

func TestSkipList_Insert(t *testing.T) {
	sl := NewSkipList()

	// Test adding first score
	now := time.Now().UTC()
	sl.Insert(1, 100, now)

	assert.Equal(t, 1, sl.GetLength())

	// Test getting rank for the user
	rank, score, exists := sl.GetRank(1)
	assert.True(t, exists)
	assert.Equal(t, uint64(1), rank)
	assert.Equal(t, uint64(100), score)

	// Test adding higher score for same user
	sl.Insert(1, 200, now)
	rank, score, exists = sl.GetRank(1)
	assert.True(t, exists)
	assert.Equal(t, uint64(1), rank)
	assert.Equal(t, uint64(200), score)

	// Test adding lower score for same user (should not replace)
	sl.Insert(1, 50, now)
	rank, score, exists = sl.GetRank(1)
	assert.True(t, exists)
	assert.Equal(t, uint64(1), rank)
	assert.Equal(t, uint64(200), score)

	// Test adding score for different user
	sl.Insert(2, 150, now)

	// Verify the ranks
	rank1, score1, exists1 := sl.GetRank(1)
	assert.True(t, exists1)
	assert.Equal(t, uint64(1), rank1)
	assert.Equal(t, uint64(200), score1)

	rank2, score2, exists2 := sl.GetRank(2)
	assert.True(t, exists2)
	assert.Equal(t, uint64(2), rank2)
	assert.Equal(t, uint64(150), score2)
}

func TestSkipList_GetTopK(t *testing.T) {
	sl := NewSkipList()
	now := time.Now().UTC()

	// Add scores for different users
	sl.Insert(1, 100, now)
	sl.Insert(2, 300, now)
	sl.Insert(3, 200, now)
	sl.Insert(4, 50, now)

	// Get top 2
	topK := sl.GetTopK(2)
	assert.Equal(t, 2, len(topK))
	assert.Equal(t, int64(2), topK[0].UserID)
	assert.Equal(t, uint64(300), topK[0].Score)
	assert.Equal(t, uint64(1), topK[0].Rank)
	assert.Equal(t, int64(3), topK[1].UserID)
	assert.Equal(t, uint64(200), topK[1].Score)
	assert.Equal(t, uint64(2), topK[1].Rank)

	// Get all (limit higher than available)
	topAll := sl.GetTopK(10)
	assert.Equal(t, 4, len(topAll))
}

func TestGameLeaderboard_GetTopK(t *testing.T) {
	gl := NewGameLeaderboard()
	now := time.Now().UTC()

	// Add scores with different timestamps
	gl.AddScore(1, 100, now.Add(-25*time.Hour)) // Outside 24h window
	gl.AddScore(2, 300, now)                    // Within 24h window
	gl.AddScore(3, 200, now)                    // Within 24h window
	gl.AddScore(4, 50, now)                     // Within 24h window

	// Get top 2 for all time
	topKAll := gl.GetTopK(2, models.AllTime)
	assert.Equal(t, 2, len(topKAll))
	assert.Equal(t, int64(2), topKAll[0].UserID)
	assert.Equal(t, uint64(300), topKAll[0].Score)
	assert.Equal(t, int64(3), topKAll[1].UserID)
	assert.Equal(t, uint64(200), topKAll[1].Score)

	// Get top 2 for last 24 hours
	topK24h := gl.GetTopK(2, models.Last24Hours)
	assert.Equal(t, 2, len(topK24h))
	assert.Equal(t, int64(2), topK24h[0].UserID)
	assert.Equal(t, uint64(300), topK24h[0].Score)
	assert.Equal(t, int64(3), topK24h[1].UserID)
	assert.Equal(t, uint64(200), topK24h[1].Score)
}

func TestGameLeaderboard_GetRankAndPercentile(t *testing.T) {
	gl := NewGameLeaderboard()
	now := time.Now().UTC()

	// Add scores for different users
	gl.AddScore(1, 100, now)
	gl.AddScore(2, 300, now)
	gl.AddScore(3, 200, now)
	gl.AddScore(4, 50, now)

	// Test rank and percentile for user 1
	rank, percentile, score, total, exists := gl.GetRankAndPercentile(1, models.AllTime)
	assert.True(t, exists)
	assert.Equal(t, uint64(3), rank)
	assert.Equal(t, uint64(100), score)
	assert.Equal(t, uint64(4), total)
	assert.InDelta(t, 25.0, percentile, 0.1) // (4-3)/4 * 100 = 25%

	// Test rank and percentile for user 2 (top)
	rank, percentile, score, total, exists = gl.GetRankAndPercentile(2, models.AllTime)
	assert.True(t, exists)
	assert.Equal(t, uint64(1), rank)
	assert.Equal(t, uint64(300), score)
	assert.Equal(t, uint64(4), total)
	assert.InDelta(t, 75.0, percentile, 0.1) // (4-1)/4 * 100 = 75%

	// Test non-existent user
	_, _, _, _, exists = gl.GetRankAndPercentile(99, models.AllTime)
	assert.False(t, exists)
}

func TestLeaderboardStore(t *testing.T) {
	store := NewLeaderboardStore()

	// Test adding scores to different games
	score1 := models.Score{GameID: 1, UserID: 1, Score: 100, Timestamp: time.Now().UTC()}
	score2 := models.Score{GameID: 1, UserID: 2, Score: 200, Timestamp: time.Now().UTC()}
	score3 := models.Score{GameID: 2, UserID: 1, Score: 300, Timestamp: time.Now().UTC()}

	store.AddScore(score1)
	store.AddScore(score2)
	store.AddScore(score3)

	// Test top leaders for game 1
	leaders1 := store.GetTopLeaders(1, 10, models.AllTime)
	assert.Equal(t, 2, len(leaders1))
	assert.Equal(t, int64(2), leaders1[0].UserID)
	assert.Equal(t, uint64(200), leaders1[0].Score)

	// Test top leaders for game 2
	leaders2 := store.GetTopLeaders(2, 10, models.AllTime)
	assert.Equal(t, 1, len(leaders2))
	assert.Equal(t, int64(1), leaders2[0].UserID)
	assert.Equal(t, uint64(300), leaders2[0].Score)

	// Test player rank for game 1
	rank, percentile, score, total, exists := store.GetPlayerRank(1, 1, models.AllTime)
	assert.True(t, exists)
	assert.Equal(t, uint64(2), rank)
	assert.Equal(t, uint64(100), score)
	assert.Equal(t, uint64(2), total)
	assert.InDelta(t, 0.0, percentile, 0.1) // (2-2)/2 * 100 = 0%

	// Test total players
	assert.Equal(t, uint64(2), store.TotalPlayers(1))
	assert.Equal(t, uint64(1), store.TotalPlayers(2))
	assert.Equal(t, uint64(0), store.TotalPlayers(99)) // Non-existent game
}
