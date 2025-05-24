package store

import (
	"log"
	"sync"

	"github.com/ringg-play/leaderboard-realtime/internal/db"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
)

// Store stores leaderboards for multiple games
type Store struct {
	mu           sync.RWMutex
	db           *db.PostgresRepository
	leaderboards map[int64]*GameLeaderboard
}

// NewStore creates a new leaderboard store
func NewStore(db *db.PostgresRepository) *Store {
	return &Store{
		leaderboards: make(map[int64]*GameLeaderboard),
		db:           db,
	}
}

// NewStoreWithWAL creates a new leaderboard store with WAL
func NewStoreWithWAL(walDir string, db *db.PostgresRepository) (*Store, error) {
	store := NewStore(db)
	return store, nil
}

func (ls *Store) GetOrCreateLeaderboard(gameID int64) *GameLeaderboard {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	leaderboard, exists := ls.leaderboards[gameID]
	if !exists {
		leaderboard = NewGameLeaderboard()
		ls.leaderboards[gameID] = leaderboard
	}

	return leaderboard
}

func (ls *Store) GetLeaderboard(gameID int64) *GameLeaderboard {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	leaderboard, exists := ls.leaderboards[gameID]

	if !exists {
		return nil
	}

	return leaderboard
}

// AddScore adds a score to the leaderboard for a specific game
func (ls *Store) AddScore(score models.Score) {
	//Save to postgres first
	// err := ls.db.SaveScore(score)
	// if err != nil {
	// 	fmt.Printf("Error saving score to PostgreSQL: %v\n", err)
	// }

	leaderboard := ls.GetOrCreateLeaderboard(score.GameID)
	// Add score to the leaderboard
	leaderboard.AddScore(score.UserID, score.Score, score.Timestamp)
}

func (ls *Store) SaveScoreBatch(scores []models.Score) error {
	for _, score := range scores {

		ls.AddScore(score)
	}
	return nil
}

// GetTopLeaders returns the top leaders for a specific game
func (ls *Store) GetTopLeaders(gameID int64, limit int, window models.TimeWindow) []models.LeaderboardEntry {
	leaderboard := ls.GetLeaderboard(gameID)
	if leaderboard == nil {
		return []models.LeaderboardEntry{}
	}
	return leaderboard.GetTopK(limit, window)
}

// GetPlayerRank returns a player's rank for a specific game
func (ls *Store) GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, bool) {
	leaderboard := ls.GetLeaderboard(gameID)
	if leaderboard == nil {
		return 0, 0, 0, 0, false
	}

	return leaderboard.GetRankAndPercentile(userID, window)
}

// TotalPlayers returns the total number of players for a specific game
func (ls *Store) TotalPlayers(gameID int64) uint64 {
	leaderboard := ls.GetLeaderboard(gameID)
	if leaderboard == nil {
		return 0
	}

	return leaderboard.TotalPlayers(models.AllTime)
}

func (ls *Store) Close() {
	log.Println("Closing store")
	return
}
