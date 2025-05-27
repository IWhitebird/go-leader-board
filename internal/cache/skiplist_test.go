package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func intCompare(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func reverseIntCompare(a, b int) int {
	if a > b {
		return -1
	}
	if a < b {
		return 1
	}
	return 0
}

func TestSkipList_Insert(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	inserted := sl.InsertOrUpdate("user1", 100)
	assert.True(t, inserted)
	assert.Equal(t, 1, sl.GetLength())

	rank, exists := sl.GetRank("user1")
	assert.True(t, exists)
	assert.Equal(t, 1, rank)

	value, found := sl.Search("user1")
	assert.True(t, found)
	assert.Equal(t, 100, value)

	// Same user submitting same score - should not change anything
	updated := sl.InsertOrUpdate("user1", 100)
	assert.False(t, updated)
	assert.Equal(t, 1, sl.GetLength())

	// Same user submitting worse score - should not update
	updated = sl.InsertOrUpdate("user1", 150) // worse score (higher number)
	assert.False(t, updated)
	assert.Equal(t, 1, sl.GetLength())

	// Same user submitting better score - should update
	updated = sl.InsertOrUpdate("user1", 50) // better score (lower number)
	assert.True(t, updated)
	assert.Equal(t, 1, sl.GetLength())

	// Verify the score was updated
	value, found = sl.Search("user1")
	assert.True(t, found)
	assert.Equal(t, 50, value)

	// Old score should be gone - search by key should still work
	value, found = sl.Search("user1")
	assert.True(t, found)
	assert.Equal(t, 50, value)
	sl.InsertOrUpdate("user2", 75)
	sl.InsertOrUpdate("user3", 25)

	assert.Equal(t, 3, sl.GetLength())

	rank1, exists1 := sl.GetRank("user3")
	assert.True(t, exists1)
	assert.Equal(t, 1, rank1)

	rank2, exists2 := sl.GetRank("user1")
	assert.True(t, exists2)
	assert.Equal(t, 2, rank2)

	rank3, exists3 := sl.GetRank("user2")
	assert.True(t, exists3)
	assert.Equal(t, 3, rank3)
}

func TestSkipList_Delete(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	sl.InsertOrUpdate("user1", 100)
	sl.InsertOrUpdate("user2", 200)
	sl.InsertOrUpdate("user3", 50)

	assert.Equal(t, 3, sl.GetLength())

	deleted := sl.Delete("user1")
	assert.True(t, deleted)
	assert.Equal(t, 2, sl.GetLength())

	_, exists := sl.Search("user1")
	assert.False(t, exists)

	deleted = sl.Delete("user4")
	assert.False(t, deleted)
	assert.Equal(t, 2, sl.GetLength())
}

func TestSkipList_GetTopK(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	sl.InsertOrUpdate("user1", 100)
	sl.InsertOrUpdate("user2", 300)
	sl.InsertOrUpdate("user3", 200)
	sl.InsertOrUpdate("user4", 50)

	topK := sl.GetTopK(2)
	assert.Equal(t, 2, len(topK))
	assert.Equal(t, "user4", topK[0].Key)
	assert.Equal(t, 50, topK[0].Value)
	assert.Equal(t, 1, topK[0].Rank)
	assert.Equal(t, "user1", topK[1].Key)
	assert.Equal(t, 100, topK[1].Value)
	assert.Equal(t, 2, topK[1].Rank)

	topAll := sl.GetTopK(10)
	assert.Equal(t, 4, len(topAll))
}

func TestSkipList_ReverseOrder(t *testing.T) {
	sl := NewSkipList[string](reverseIntCompare)

	sl.InsertOrUpdate("user1", 100)
	sl.InsertOrUpdate("user2", 300)
	sl.InsertOrUpdate("user3", 200)
	sl.InsertOrUpdate("user4", 50)

	topK := sl.GetTopK(2)
	assert.Equal(t, 2, len(topK))
	assert.Equal(t, "user2", topK[0].Key)
	assert.Equal(t, 300, topK[0].Value)
	assert.Equal(t, 1, topK[0].Rank)
	assert.Equal(t, "user3", topK[1].Key)
	assert.Equal(t, 200, topK[1].Value)
	assert.Equal(t, 2, topK[1].Rank)
}

func TestSkipList_Contains(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	assert.False(t, sl.Contains("user1"))

	sl.InsertOrUpdate("user1", 100)
	assert.True(t, sl.Contains("user1"))

	sl.Delete("user1")
	assert.False(t, sl.Contains("user1"))
}

func TestSkipList_IsEmpty(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	assert.True(t, sl.IsEmpty())

	sl.InsertOrUpdate("user1", 100)
	assert.False(t, sl.IsEmpty())

	sl.Delete("user1")
	assert.True(t, sl.IsEmpty())
}

func TestSkipList_Clear(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	sl.InsertOrUpdate("user1", 100)
	sl.InsertOrUpdate("user2", 200)
	sl.InsertOrUpdate("user3", 300)

	assert.Equal(t, 3, sl.GetLength())
	assert.False(t, sl.IsEmpty())

	sl.Clear()

	assert.Equal(t, 0, sl.GetLength())
	assert.True(t, sl.IsEmpty())
	assert.False(t, sl.Contains("user1"))
}

func TestSkipList_GetAll(t *testing.T) {
	sl := NewSkipList[string](intCompare)

	sl.InsertOrUpdate("user3", 300)
	sl.InsertOrUpdate("user1", 100)
	sl.InsertOrUpdate("user2", 200)

	all := sl.GetAll()
	assert.Equal(t, 3, len(all))

	assert.Equal(t, "user1", all[0].Key)
	assert.Equal(t, 100, all[0].Value)
	assert.Equal(t, 1, all[0].Rank)

	assert.Equal(t, "user2", all[1].Key)
	assert.Equal(t, 200, all[1].Value)
	assert.Equal(t, 2, all[1].Rank)

	assert.Equal(t, "user3", all[2].Key)
	assert.Equal(t, 300, all[2].Value)
	assert.Equal(t, 3, all[2].Rank)
}

func TestSkipList_GetRank(t *testing.T) {
	sl := NewSkipList[int, int](func(a, b int) int {
		if a > b {
			return -1
		} else if a < b {
			return 1
		}
		return 0
	})

	// Insert values in random order
	values := []int{50, 100, 25, 75, 10, 90, 30}
	for i, val := range values {
		sl.InsertOrUpdate(i, val)
	}

	// Test ranks (sorted order should be: 100, 90, 75, 50, 30, 25, 10)
	rank, found := sl.GetRank(1) // value 100
	assert.True(t, found)
	assert.Equal(t, 1, rank)

	rank, found = sl.GetRank(5) // value 90
	assert.True(t, found)
	assert.Equal(t, 2, rank)

	rank, found = sl.GetRank(3) // value 75
	assert.True(t, found)
	assert.Equal(t, 3, rank)

	rank, found = sl.GetRank(0) // value 50
	assert.True(t, found)
	assert.Equal(t, 4, rank)

	rank, found = sl.GetRank(6) // value 30
	assert.True(t, found)
	assert.Equal(t, 5, rank)

	rank, found = sl.GetRank(2) // value 25
	assert.True(t, found)
	assert.Equal(t, 6, rank)

	rank, found = sl.GetRank(4) // value 10
	assert.True(t, found)
	assert.Equal(t, 7, rank)

	// Test non-existent key
	rank, found = sl.GetRank(999)
	assert.False(t, found)
	assert.Equal(t, 0, rank)
}
