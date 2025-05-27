package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/IWhitebird/go-leader-board/config"
	"github.com/IWhitebird/go-leader-board/internal/db"
	"github.com/IWhitebird/go-leader-board/internal/logging"
	"github.com/IWhitebird/go-leader-board/internal/models"
)

type Store struct {
	mu           sync.RWMutex
	db           *db.PostgresRepository
	leaderboards map[int64]*GameLeaderboard
}

func NewStore(db *db.PostgresRepository) *Store {
	store := &Store{
		leaderboards: make(map[int64]*GameLeaderboard),
		db:           db,
	}
	// For now let's not run the cleanup.
	// store.StartPeriodicCleanup()
	return store
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
	leaderboard, exists := ls.leaderboards[gameID]
	if !exists {
		return nil
	}
	return leaderboard
}

func (ls *Store) AddScore(score models.Score) error {
	if ls.db != nil {
		err := ls.db.SaveScore(score)
		if err != nil {
			return fmt.Errorf("failed to save score to PostgreSQL: %w", err)
		}
	}

	ls.addScoreToCache(score)
	return nil
}

func (ls *Store) SaveScoreBatch(scores []models.Score) error {
	if len(scores) == 0 {
		return nil
	}

	if ls.db != nil {
		err := ls.db.SaveScoreBatch(scores)
		if err != nil {
			return fmt.Errorf("failed to save scores to PostgreSQL: %w", err)
		}
	}

	for _, score := range scores {
		ls.addScoreToCache(score)
	}

	return nil
}

func (ls *Store) addScoreToCache(score models.Score) {
	leaderboard := ls.GetOrCreateLeaderboard(score.GameID)
	leaderboard.AddScore(score.UserID, score.Score, score.Timestamp)
}

func (ls *Store) GetTopLeaders(gameID int64, limit int, window models.TimeWindow) []models.LeaderboardEntry {
	leaderboard := ls.GetLeaderboard(gameID)
	if leaderboard == nil {
		return []models.LeaderboardEntry{}
	}
	return leaderboard.GetTopK(limit, window)
}

func (ls *Store) GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, bool) {
	leaderboard := ls.GetLeaderboard(gameID)
	if leaderboard == nil {
		return 0, 0, 0, 0, false
	}
	return leaderboard.GetRankAndPercentile(userID, window)
}

func (ls *Store) TotalPlayers(gameID int64) uint64 {
	leaderboard := ls.GetLeaderboard(gameID)
	if leaderboard == nil {
		return 0
	}
	return leaderboard.TotalPlayers(models.AllTime)
}

func (ls *Store) InitializeFromDatabase(cfg *config.AppConfig) error {
	games, err := ls.db.GetAllGames()
	if err != nil {
		return fmt.Errorf("failed to load scores from database: %w", err)
	}

	logging.Info("Initializing store with", len(games), "games")
	for _, gameID := range games {
		go ls.CacheGameLeaderboard(gameID)
	}

	return nil
}

func (ls *Store) CacheGameLeaderboard(gameID int64) error {
	scores, err := ls.db.GetAllScoresForGame(gameID)
	if err != nil {
		return fmt.Errorf("failed to load scores for game %d: %w", gameID, err)
	}

	leaderboard := ls.GetOrCreateLeaderboard(gameID)
	leaderboard.AddScoreBatch(scores)
	return nil
}

func (ls *Store) CleanOldEntries() {
	// ls.mu.RLock()
	// defer ls.mu.RUnlock()

	for _, leaderboard := range ls.leaderboards {
		leaderboard.CleanOldEntries()
	}
}

func (ls *Store) StartPeriodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			ls.CleanOldEntries()
		}
	}()
}

func (ls *Store) Close() {
	return
}
