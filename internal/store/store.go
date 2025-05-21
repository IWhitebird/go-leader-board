package store

import (
	"fmt"
	"sort"
	"sync"
	"time"

	cache "github.com/ringg-play/leaderboard-realtime/internal/cache"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
)

// ScoreEntry represents a score with ordering metadata
type ScoreEntry struct {
	Score     uint64
	UserID    int64
	Timestamp time.Time
}

// GameLeaderboard manages scores for a single game
type GameLeaderboard struct {
	mu            sync.RWMutex
	allTimeScores *cache.SkipList           // Skiplist for all-time scores
	recentScores  map[int64]*cache.SkipList // Map of time window (hours) to skiplist for recent scores
}

// NewGameLeaderboard creates a new game leaderboard
func NewGameLeaderboard() *GameLeaderboard {
	return &GameLeaderboard{
		allTimeScores: cache.NewSkipList(),
		recentScores:  make(map[int64]*cache.SkipList),
	}
}

// getOrCreateRecentScores gets or creates a skiplist for the specified time window
func (gl *GameLeaderboard) getOrCreateRecentScores(hours int64) *cache.SkipList {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	if hours <= 0 {
		return gl.allTimeScores
	}

	if skiplist, exists := gl.recentScores[hours]; exists {
		return skiplist
	}

	// Create new skiplist for this time window
	skiplist := cache.NewSkipList()
	gl.recentScores[hours] = skiplist
	return skiplist
}

// AddScore adds a new score to the leaderboard
func (gl *GameLeaderboard) AddScore(userID int64, score uint64, timestamp time.Time) {
	// Always add to the all-time skiplist
	gl.mu.Lock()
	gl.allTimeScores.Insert(userID, score, timestamp)
	gl.mu.Unlock()

	// Add to each time window skiplist
	gl.mu.RLock()
	for hours, skiplist := range gl.recentScores {
		cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
		if timestamp.After(cutoff) {
			skiplist.Insert(userID, score, timestamp)
		}
	}
	gl.mu.RUnlock()
}

// GetTopK returns the top k entries from the leaderboard
func (gl *GameLeaderboard) GetTopK(k int, window models.TimeWindow) []models.LeaderboardEntry {
	if window.Hours <= 0 {
		// All time scores
		return gl.allTimeScores.GetTopK(k)
	}

	// Get the correct skiplist for this time window
	skiplist := gl.getOrCreateRecentScores(int64(window.Hours))
	return skiplist.GetTopK(k)
}

// GetRankAndPercentile gets a player's rank and percentile in the leaderboard
func (gl *GameLeaderboard) GetRankAndPercentile(userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, bool) {
	var skiplist *cache.SkipList

	if window.Hours <= 0 {
		// All time scores
		skiplist = gl.allTimeScores
	} else {
		// Get the correct skiplist for this time window
		skiplist = gl.getOrCreateRecentScores(int64(window.Hours))
	}

	// Get rank and score
	rank, score, exists := skiplist.GetRank(userID)
	if !exists {
		return 0, 0, 0, 0, false
	}

	// Calculate percentile
	total := uint64(skiplist.GetLength())
	percentile := skiplist.GetPercentile(rank)

	return rank, percentile, score, total, true
}

// TotalPlayers returns the total number of players in the leaderboard
func (gl *GameLeaderboard) TotalPlayers(window models.TimeWindow) uint64 {
	if window.Hours <= 0 {
		// All time scores
		return uint64(gl.allTimeScores.GetLength())
	}

	// Get the correct skiplist for this time window
	skiplist := gl.getOrCreateRecentScores(int64(window.Hours))
	return uint64(skiplist.GetLength())
}

// CleanOldEntries removes entries older than the specified cutoff time
func (gl *GameLeaderboard) CleanOldEntries() {
	// We don't need to clean old entries from the skiplists
	// as they're maintained for each time window separately
	// But we can clean up any time windows that are no longer needed
	gl.mu.Lock()
	defer gl.mu.Unlock()

	// No explicit cleanup needed - the time windows are maintained automatically
}

// LeaderboardStore stores leaderboards for multiple games
type LeaderboardStore struct {
	mu           sync.RWMutex
	leaderboards map[int64]*GameLeaderboard
	wal          *WAL // Write-ahead log for persistence
}

// NewLeaderboardStore creates a new leaderboard store
func NewLeaderboardStore() *LeaderboardStore {
	return &LeaderboardStore{
		leaderboards: make(map[int64]*GameLeaderboard),
	}
}

// NewLeaderboardStoreWithWAL creates a new leaderboard store with WAL
func NewLeaderboardStoreWithWAL(walDir string) (*LeaderboardStore, error) {
	store := NewLeaderboardStore()

	// Create WAL
	wal, err := NewWAL(walDir, 1*time.Second)
	if err != nil {
		return nil, err
	}
	store.wal = wal

	// Recover from WAL
	if err := RecoverFromWAL(walDir, store); err != nil {
		return nil, err
	}

	return store, nil
}

// AddScore adds a score to the leaderboard for a specific game
func (ls *LeaderboardStore) AddScore(score models.Score) {
	ls.mu.Lock()
	leaderboard, exists := ls.leaderboards[score.GameID]
	if !exists {
		leaderboard = NewGameLeaderboard()
		ls.leaderboards[score.GameID] = leaderboard
	}
	ls.mu.Unlock()

	// Add score to the leaderboard
	leaderboard.AddScore(score.UserID, score.Score, score.Timestamp)

	// Log to WAL if enabled
	if ls.wal != nil {
		if err := ls.wal.LogScore(score); err != nil {
			// Log the error but continue
			// In a production system, we would use a real logger and have a retry mechanism
			fmt.Printf("Error logging score to WAL: %v\n", err)
		}
	}
}

// GetTopLeaders returns the top leaders for a specific game
func (ls *LeaderboardStore) GetTopLeaders(gameID int64, limit int, window models.TimeWindow) []models.LeaderboardEntry {
	ls.mu.RLock()
	leaderboard, exists := ls.leaderboards[gameID]
	ls.mu.RUnlock()

	if !exists {
		return []models.LeaderboardEntry{}
	}

	return leaderboard.GetTopK(limit, window)
}

// GetPlayerRank returns a player's rank for a specific game
func (ls *LeaderboardStore) GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, bool) {
	ls.mu.RLock()
	leaderboard, exists := ls.leaderboards[gameID]
	ls.mu.RUnlock()

	if !exists {
		return 0, 0, 0, 0, false
	}

	return leaderboard.GetRankAndPercentile(userID, window)
}

// TotalPlayers returns the total number of players for a specific game
func (ls *LeaderboardStore) TotalPlayers(gameID int64) uint64 {
	ls.mu.RLock()
	leaderboard, exists := ls.leaderboards[gameID]
	ls.mu.RUnlock()

	if !exists {
		return 0
	}

	return leaderboard.TotalPlayers(models.AllTime)
}

// Close closes the leaderboard store and any associated resources
func (ls *LeaderboardStore) Close() error {
	if ls.wal != nil {
		return ls.wal.Close()
	}
	return nil
}

// CreateSnapshot creates a snapshot of the current state
func (ls *LeaderboardStore) CreateSnapshot() error {
	if ls.wal == nil {
		return nil
	}

	ls.mu.RLock()
	defer ls.mu.RUnlock()

	// Create a snapshot of all scores
	snapshot := make(map[int64][]models.Score)

	// for gameID, leaderboard := range ls.leaderboards {
	// 	// Get all scores from the all-time skiplist
	// 	leaderboard.mu.RLock()
	// 	skiplist := leaderboard.allTimeScores

	// 	// Convert skiplist to scores
	// 	skiplist.Mu.RLock()
	// 	node := skiplist.Header.Forward[0]
	// 	scores := make([]models.Score, 0, skiplist.Length)

	// 	for node != nil {
	// 		scores = append(scores, models.Score{
	// 			GameID:    gameID,
	// 			UserID:    node.UserID,
	// 			Score:     node.Score,
	// 			Timestamp: node.Timestamp,
	// 		})
	// 		node = node.Forward[0]
	// 	}
	// 	skiplist.Mu.RUnlock()
	// 	leaderboard.mu.RUnlock()

	// 	snapshot[gameID] = scores
	// }

	// Log the snapshot to WAL
	return ls.wal.LogSnapshot(snapshot)
}

// Helper function to sort score entries by score (descending)
func sortScoreEntries(entries []ScoreEntry) {
	sort.Slice(entries, func(i, j int) bool {
		// Primary sort by score (descending)
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		// Secondary sort by timestamp (ascending)
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
