package cache

import (
	"math/rand"
	"sync"
	"time"

	"github.com/ringg-play/leaderboard-realtime/internal/models"
)

const (
	MaxLevel = 16
	P        = 0.5
)

type SkipListNode struct {
	Score     uint64
	UserID    int64
	Timestamp time.Time
	Forward   []*SkipListNode
}

type SkipList struct {
	// Mu        sync.RWMutex
	// Length    int
	// Header    *SkipListNode

	mu     sync.RWMutex
	length int
	header *SkipListNode

	level     int
	userIndex map[int64]*SkipListNode
	rand      *rand.Rand
}

func NewSkipList() *SkipList {
	header := &SkipListNode{
		Forward: make([]*SkipListNode, MaxLevel),
	}

	return &SkipList{
		header:    header,
		level:     1,
		userIndex: make(map[int64]*SkipListNode),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// randomLevel generates a random level for a new node
func (sl *SkipList) randomLevel() int {
	level := 1
	for level < MaxLevel && sl.rand.Float64() < P {
		level++
	}
	return level
}

// Insert adds or updates a score in the skiplist
func (sl *SkipList) Insert(userID int64, score uint64, timestamp time.Time) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Check if user already exists
	if existing, exists := sl.userIndex[userID]; exists {
		// If the new score is not better, don't update
		if existing.Score >= score {
			return false
		}
		// Otherwise, remove the existing node
		sl.deleteNode(userID, existing.Score)
	}

	// Create update array to track changes at each level
	update := make([]*SkipListNode, MaxLevel)
	x := sl.header

	// Start from highest level and work down
	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && (x.Forward[i].Score > score ||
			(x.Forward[i].Score == score && x.Forward[i].UserID < userID)) {
			x = x.Forward[i]
		}
		update[i] = x
	}

	// Generate random level for new node
	newLevel := sl.randomLevel()
	if newLevel > sl.level {
		for i := sl.level; i < newLevel; i++ {
			update[i] = sl.header
		}
		sl.level = newLevel
	}

	// Create new node
	newNode := &SkipListNode{
		Score:     score,
		UserID:    userID,
		Timestamp: timestamp,
		Forward:   make([]*SkipListNode, newLevel),
	}

	// Insert the node at all levels
	for i := 0; i < newLevel; i++ {
		newNode.Forward[i] = update[i].Forward[i]
		update[i].Forward[i] = newNode
	}

	// Update user index
	sl.userIndex[userID] = newNode
	sl.length++
	return true
}

// deleteNode removes a node from the skiplist
func (sl *SkipList) deleteNode(userID int64, score uint64) {
	update := make([]*SkipListNode, MaxLevel)
	x := sl.header

	// Find node at each level
	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil &&
			(x.Forward[i].Score > score ||
				(x.Forward[i].Score == score && x.Forward[i].UserID < userID)) {
			x = x.Forward[i]
		}
		update[i] = x
	}

	// First node in the level is the candidate for removal
	x = x.Forward[0]
	if x != nil && x.UserID == userID && x.Score == score {
		// Remove node at each level
		for i := 0; i < sl.level; i++ {
			if update[i].Forward[i] != x {
				break
			}
			update[i].Forward[i] = x.Forward[i]
		}

		// Update the level of the skiplist if needed
		for sl.level > 1 && sl.header.Forward[sl.level-1] == nil {
			sl.level--
		}

		// Remove from user index
		delete(sl.userIndex, userID)
		sl.length--
	}
}

// GetRank returns the rank of a user (1-based, highest score = rank 1)
func (sl *SkipList) GetRank(userID int64) (uint64, uint64, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	// Check if user exists
	node, exists := sl.userIndex[userID]
	if !exists {
		return 0, 0, false
	}

	// Count nodes with higher scores
	var rank uint64 = 1
	x := sl.header

	// Start from highest level and work down
	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && (x.Forward[i].Score > node.Score ||
			(x.Forward[i].Score == node.Score && x.Forward[i].UserID < node.UserID)) {
			rank++
			x = x.Forward[i]
		}
	}

	return rank, node.Score, true
}

// GetTopK returns the top K entries from the skiplist
func (sl *SkipList) GetTopK(k int) []models.LeaderboardEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	result := make([]models.LeaderboardEntry, 0, k)
	x := sl.header.Forward[0]

	// Get the top k entries
	for i := 0; i < k && x != nil; i++ {
		result = append(result, models.LeaderboardEntry{
			UserID: x.UserID,
			Score:  x.Score,
			Rank:   uint64(i + 1),
		})
		x = x.Forward[0]
	}

	return result
}

// GetLength returns the number of entries in the skiplist
func (sl *SkipList) GetLength() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.length
}

// Contains checks if a user exists in the skiplist
func (sl *SkipList) Contains(userID int64) bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	_, exists := sl.userIndex[userID]
	return exists
}

// GetPercentile calculates the percentile of a user
func (sl *SkipList) GetPercentile(rank uint64) float64 {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if sl.length == 0 {
		return 0.0
	}

	return 100.0 * float64(sl.length-int(rank)) / float64(sl.length)
}
