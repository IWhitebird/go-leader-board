package cache

import (
	"testing"
	"time"

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
