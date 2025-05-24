package store

import (
	"fmt"
	"log"
	"sync"

	"github.com/ringg-play/leaderboard-realtime/config"
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
	// ls.mu.RLock()
	// defer ls.mu.RUnlock()

	leaderboard, exists := ls.leaderboards[gameID]

	if !exists {
		return nil
	}

	return leaderboard
}

// AddScore adds a score to both PostgreSQL and cache (for single score operations)
func (ls *Store) AddScore(score models.Score) error {
	// Save to PostgreSQL first
	err := ls.db.SaveScore(score)
	if err != nil {
		return fmt.Errorf("failed to save score to PostgreSQL: %w", err)
	}

	// After successful PostgreSQL save, update the cache
	ls.addScoreToCache(score)

	return nil
}

func (ls *Store) SaveScoreBatch(scores []models.Score) error {
	if len(scores) == 0 {
		return nil
	}

	// Save to PostgreSQL first for consistency
	err := ls.db.SaveScoreBatch(scores)
	if err != nil {
		return fmt.Errorf("failed to save scores to PostgreSQL: %w", err)
	}

	// After successful PostgreSQL save, update the cache
	for _, score := range scores {
		ls.addScoreToCache(score)
	}

	log.Printf("Successfully saved batch of %d scores to PostgreSQL and updated cache", len(scores))
	return nil
}

// addScoreToCache adds a score only to the cache (used after PostgreSQL save or during initialization)
func (ls *Store) addScoreToCache(score models.Score) {
	leaderboard := ls.GetOrCreateLeaderboard(score.GameID)
	leaderboard.AddScore(score.UserID, score.Score, score.Timestamp)
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

// InitializeFromDatabase loads all existing scores from PostgreSQL and populates the cache
func (ls *Store) InitializeFromDatabase(cfg *config.AppConfig) error {
	log.Println("Initializing store from PostgreSQL database...")

	games, err := ls.db.GetAllGames()
	if err != nil {
		return fmt.Errorf("failed to load scores from database: %w", err)
	}

	for _, gameID := range games {
		go ls.CacheGameLeaderboard(gameID)
	}

	log.Printf("Successfully initialized store with scores from %d games", len(games))
	return nil
}

func (ls *Store) CacheGameLeaderboard(gameID int64) error {
	log.Printf("Initializing store from PostgreSQL database for game %d...", gameID)

	scores, err := ls.db.GetAllScoresForGame(gameID)
	if err != nil {
		return fmt.Errorf("failed to load scores for game %d: %w", gameID, err)
	}

	leaderboard := ls.GetOrCreateLeaderboard(gameID)
	leaderboard.AddScoreBatch(scores)

	return nil
}

func (ls *Store) Close() {
	log.Println("Closing store")
	return
}
