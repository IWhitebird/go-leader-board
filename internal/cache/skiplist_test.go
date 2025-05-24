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
	sl := NewSkipList[int, string](intCompare)

	inserted := sl.Insert(100, "user1")
	assert.True(t, inserted)
	assert.Equal(t, 1, sl.GetLength())

	rank, exists := sl.GetRank(100)
	assert.True(t, exists)
	assert.Equal(t, 1, rank)

	value, found := sl.Search(100)
	assert.True(t, found)
	assert.Equal(t, "user1", value)

	updated := sl.Insert(100, "user1_updated")
	assert.False(t, updated)
	assert.Equal(t, 1, sl.GetLength())

	value, found = sl.Search(100)
	assert.True(t, found)
	assert.Equal(t, "user1_updated", value)

	sl.Insert(50, "user2")
	sl.Insert(150, "user3")

	assert.Equal(t, 3, sl.GetLength())

	rank1, exists1 := sl.GetRank(50)
	assert.True(t, exists1)
	assert.Equal(t, 1, rank1)

	rank2, exists2 := sl.GetRank(100)
	assert.True(t, exists2)
	assert.Equal(t, 2, rank2)

	rank3, exists3 := sl.GetRank(150)
	assert.True(t, exists3)
	assert.Equal(t, 3, rank3)
}

func TestSkipList_Delete(t *testing.T) {
	sl := NewSkipList[int, string](intCompare)

	sl.Insert(100, "user1")
	sl.Insert(200, "user2")
	sl.Insert(50, "user3")

	assert.Equal(t, 3, sl.GetLength())

	deleted := sl.Delete(100)
	assert.True(t, deleted)
	assert.Equal(t, 2, sl.GetLength())

	_, exists := sl.Search(100)
	assert.False(t, exists)

	deleted = sl.Delete(300)
	assert.False(t, deleted)
	assert.Equal(t, 2, sl.GetLength())
}

func TestSkipList_GetTopK(t *testing.T) {
	sl := NewSkipList[int, string](intCompare)

	sl.Insert(100, "user1")
	sl.Insert(300, "user2")
	sl.Insert(200, "user3")
	sl.Insert(50, "user4")

	topK := sl.GetTopK(2)
	assert.Equal(t, 2, len(topK))
	assert.Equal(t, 50, topK[0].Key)
	assert.Equal(t, "user4", topK[0].Value)
	assert.Equal(t, 1, topK[0].Rank)
	assert.Equal(t, 100, topK[1].Key)
	assert.Equal(t, "user1", topK[1].Value)
	assert.Equal(t, 2, topK[1].Rank)

	topAll := sl.GetTopK(10)
	assert.Equal(t, 4, len(topAll))
}

func TestSkipList_ReverseOrder(t *testing.T) {
	sl := NewSkipList[int, string](reverseIntCompare)

	sl.Insert(100, "user1")
	sl.Insert(300, "user2")
	sl.Insert(200, "user3")
	sl.Insert(50, "user4")

	topK := sl.GetTopK(2)
	assert.Equal(t, 2, len(topK))
	assert.Equal(t, 300, topK[0].Key)
	assert.Equal(t, "user2", topK[0].Value)
	assert.Equal(t, 1, topK[0].Rank)
	assert.Equal(t, 200, topK[1].Key)
	assert.Equal(t, "user3", topK[1].Value)
	assert.Equal(t, 2, topK[1].Rank)
}

func TestSkipList_Contains(t *testing.T) {
	sl := NewSkipList[int, string](intCompare)

	assert.False(t, sl.Contains(100))

	sl.Insert(100, "user1")
	assert.True(t, sl.Contains(100))

	sl.Delete(100)
	assert.False(t, sl.Contains(100))
}

func TestSkipList_IsEmpty(t *testing.T) {
	sl := NewSkipList[int, string](intCompare)

	assert.True(t, sl.IsEmpty())

	sl.Insert(100, "user1")
	assert.False(t, sl.IsEmpty())

	sl.Delete(100)
	assert.True(t, sl.IsEmpty())
}

func TestSkipList_Clear(t *testing.T) {
	sl := NewSkipList[int, string](intCompare)

	sl.Insert(100, "user1")
	sl.Insert(200, "user2")
	sl.Insert(300, "user3")

	assert.Equal(t, 3, sl.GetLength())
	assert.False(t, sl.IsEmpty())

	sl.Clear()

	assert.Equal(t, 0, sl.GetLength())
	assert.True(t, sl.IsEmpty())
	assert.False(t, sl.Contains(100))
}

func TestSkipList_GetAll(t *testing.T) {
	sl := NewSkipList[int, string](intCompare)

	sl.Insert(300, "user3")
	sl.Insert(100, "user1")
	sl.Insert(200, "user2")

	all := sl.GetAll()
	assert.Equal(t, 3, len(all))

	assert.Equal(t, 100, all[0].Key)
	assert.Equal(t, "user1", all[0].Value)
	assert.Equal(t, 1, all[0].Rank)

	assert.Equal(t, 200, all[1].Key)
	assert.Equal(t, "user2", all[1].Value)
	assert.Equal(t, 2, all[1].Rank)

	assert.Equal(t, 300, all[2].Key)
	assert.Equal(t, "user3", all[2].Value)
	assert.Equal(t, 3, all[2].Rank)
}
