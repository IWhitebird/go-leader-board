package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ringg-play/leaderboard-realtime/internal/models"
)

// PersistenceManager manages persistence of scores to disk
type PersistenceManager struct {
	dataDir string
	store   *LeaderboardStore
	mutex   sync.Mutex
}

// NewPersistenceManager creates a new persistence manager
func NewPersistenceManager(dataDir string, store *LeaderboardStore) (*PersistenceManager, error) {
	pm := &PersistenceManager{
		dataDir: dataDir,
		store:   store,
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load any existing data
	if err := pm.LoadScores(); err != nil {
		return nil, fmt.Errorf("failed to load scores: %w", err)
	}

	// Start background saving
	go pm.startPeriodicSave()

	return pm, nil
}

// SaveScores saves all scores to disk
func (pm *PersistenceManager) SaveScores() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Create a snapshot file with timestamp
	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("leaderboard-snapshot-%s.json", timestamp)
	filePath := filepath.Join(pm.dataDir, filename)

	// Create temp file with the scores from each game
	tempPath := filePath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	// Get all game IDs from the store
	var allGameIDs []int64
	pm.store.mu.RLock()
	for gameID := range pm.store.leaderboards {
		allGameIDs = append(allGameIDs, gameID)
	}
	pm.store.mu.RUnlock()

	// Write header
	header := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"version":   "1.0",
		"games":     allGameIDs,
	}

	if err := encoder.Encode(header); err != nil {
		return fmt.Errorf("failed to encode header: %w", err)
	}

	// Write scores for each game
	for _, gameID := range allGameIDs {
		scores, err := getScoresForGame(pm.store, gameID)
		if err != nil {
			return fmt.Errorf("failed to get scores for game %d: %w", gameID, err)
		}

		// Write scores
		gameData := map[string]interface{}{
			"game_id": gameID,
			"scores":  scores,
		}

		if err := encoder.Encode(gameData); err != nil {
			return fmt.Errorf("failed to encode game data: %w", err)
		}
	}

	// Flush and rename the temp file
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	log.Printf("Saved leaderboard snapshot to %s", filePath)
	return nil
}

// getScoresForGame extracts all scores for a game from the leaderboard store
func getScoresForGame(store *LeaderboardStore, gameID int64) ([]models.Score, error) {
	store.mu.RLock()
	leaderboard, exists := store.leaderboards[gameID]
	store.mu.RUnlock()

	if !exists {
		return []models.Score{}, nil
	}

	// Access the skiplist and extract all nodes
	leaderboard.mu.RLock()
	defer leaderboard.mu.RUnlock()

	return nil, nil

	// skiplist := leaderboard.allTimeScores

	// // Lock the skiplist
	// skiplist.Mu.RLock()
	// defer skiplist.Mu.RUnlock()

	// // Extract all scores
	// scores := make([]models.Score, 0, skiplist.Length)
	// node := skiplist.Header.Forward[0]

	// for node != nil {
	// 	scores = append(scores, models.Score{
	// 		GameID:    gameID,
	// 		UserID:    node.UserID,
	// 		Score:     node.Score,
	// 		Timestamp: node.Timestamp,
	// 	})
	// 	node = node.Forward[0]
	// }

	// return scores, nil
}

// LoadScores loads scores from the most recent snapshot
func (pm *PersistenceManager) LoadScores() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Find the most recent snapshot
	pattern := filepath.Join(pm.dataDir, "leaderboard-snapshot-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob for snapshots: %w", err)
	}

	if len(matches) == 0 {
		log.Println("No snapshots found to load")
		return nil
	}

	// Find the most recent file (based on filename pattern which includes timestamp)
	var mostRecent string
	for _, match := range matches {
		if mostRecent == "" || match > mostRecent {
			mostRecent = match
		}
	}

	log.Printf("Loading scores from %s", mostRecent)

	// Open the file
	file, err := os.Open(mostRecent)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Read header
	var header map[string]interface{}
	if err := decoder.Decode(&header); err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}

	// Read game data
	for decoder.More() {
		var gameData map[string]json.RawMessage
		if err := decoder.Decode(&gameData); err != nil {
			return fmt.Errorf("failed to decode game data: %w", err)
		}

		// Parse game ID
		var gameID int64
		if err := json.Unmarshal(gameData["game_id"], &gameID); err != nil {
			return fmt.Errorf("failed to unmarshal game ID: %w", err)
		}

		// Parse scores
		var scores []models.Score
		if err := json.Unmarshal(gameData["scores"], &scores); err != nil {
			return fmt.Errorf("failed to unmarshal scores: %w", err)
		}

		// Add scores to store
		for _, score := range scores {
			pm.store.AddScore(score)
		}

		log.Printf("Loaded %d scores for game %d", len(scores), gameID)
	}

	return nil
}

// startPeriodicSave starts a goroutine that periodically saves scores
func (pm *PersistenceManager) startPeriodicSave() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := pm.SaveScores(); err != nil {
			log.Printf("Failed to save scores: %v", err)
		}
	}
}
