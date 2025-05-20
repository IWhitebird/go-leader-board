package db

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ringg-play/leaderboard-realtime/models"
)

const (
	// WALFilePrefix is the prefix for WAL files
	WALFilePrefix = "leaderboard-wal-"
	// WALFileSuffix is the suffix for WAL files
	WALFileSuffix = ".log"
	// MaxWALSize is the maximum size of a WAL file before rotation (10MB)
	MaxWALSize = 10 * 1024 * 1024
)

// WALEntry represents an entry in the Write-Ahead Log
type WALEntry struct {
	Type      string      `json:"type"`      // Type of entry (e.g., "score", "snapshot")
	Timestamp time.Time   `json:"timestamp"` // When the entry was created
	Data      interface{} `json:"data"`      // The actual data
}

// WAL implements a Write-Ahead Log for durability
type WAL struct {
	mu         sync.Mutex
	dir        string        // Directory for WAL files
	file       *os.File      // Current WAL file
	fileSize   int64         // Current WAL file size
	writer     *bufio.Writer // Buffered writer for efficiency
	autoCommit time.Duration // Interval for auto-committing
	stop       chan struct{} // Channel to signal stop
}

// NewWAL creates a new Write-Ahead Log
func NewWAL(dir string, autoCommit time.Duration) (*WAL, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Create a new WAL
	wal := &WAL{
		dir:        dir,
		autoCommit: autoCommit,
		stop:       make(chan struct{}),
	}

	// Create or open the WAL file
	if err := wal.createOrOpenFile(); err != nil {
		return nil, err
	}

	// Start auto-commit if enabled
	if autoCommit > 0 {
		go wal.autoCommitLoop()
	}

	return wal, nil
}

// createOrOpenFile creates or opens the WAL file
func (w *WAL) createOrOpenFile() error {
	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s%s%s", WALFilePrefix, timestamp, WALFileSuffix)
	path := filepath.Join(w.dir, filename)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open WAL file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat WAL file: %w", err)
	}

	// Close old file if there is one
	if w.file != nil {
		if err := w.writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush WAL writer: %w", err)
		}
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close old WAL file: %w", err)
		}
	}

	w.file = file
	w.fileSize = info.Size()
	w.writer = bufio.NewWriter(file)

	return nil
}

// LogScore logs a score to the WAL
func (w *WAL) LogScore(score models.Score) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := WALEntry{
		Type:      "score",
		Timestamp: time.Now().UTC(),
		Data:      score,
	}

	// Check if we need to rotate the file
	if w.fileSize >= MaxWALSize {
		if err := w.createOrOpenFile(); err != nil {
			return err
		}
	}

	// Serialize the entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry: %w", err)
	}

	// Write the entry
	data = append(data, '\n')
	n, err := w.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write WAL entry: %w", err)
	}

	w.fileSize += int64(n)
	return nil
}

// LogSnapshot logs a snapshot to the WAL
func (w *WAL) LogSnapshot(snapshot map[int64][]models.Score) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := WALEntry{
		Type:      "snapshot",
		Timestamp: time.Now().UTC(),
		Data:      snapshot,
	}

	// Create a new file for the snapshot
	if err := w.createOrOpenFile(); err != nil {
		return err
	}

	// Serialize the entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot entry: %w", err)
	}

	// Write the entry
	data = append(data, '\n')
	n, err := w.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write snapshot entry: %w", err)
	}

	w.fileSize += int64(n)
	return nil
}

// Sync flushes the WAL to disk
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL writer: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL file: %w", err)
	}

	return nil
}

// autoCommitLoop runs the auto-commit loop
func (w *WAL) autoCommitLoop() {
	ticker := time.NewTicker(w.autoCommit)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.Sync(); err != nil {
				// Log error, but continue
				fmt.Printf("Error syncing WAL: %v\n", err)
			}
		case <-w.stop:
			return
		}
	}
}

// Close closes the WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.autoCommit > 0 {
		close(w.stop)
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL writer: %w", err)
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close WAL file: %w", err)
	}

	return nil
}

// RecoverFromWAL recovers the state from WAL files
func RecoverFromWAL(dir string, store *LeaderboardStore) error {
	// Find all WAL files
	pattern := filepath.Join(dir, WALFilePrefix+"*"+WALFileSuffix)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob WAL files: %w", err)
	}

	if len(files) == 0 {
		return nil // No WAL files to recover from
	}

	// Sort WAL files by name (which includes timestamp)
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i] > files[j] {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Process each WAL file in order
	for _, file := range files {
		if err := processWALFile(file, store); err != nil {
			return fmt.Errorf("failed to process WAL file %s: %w", file, err)
		}
	}

	return nil
}

// processWALFile processes a single WAL file
func processWALFile(path string, store *LeaderboardStore) error {
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer file.Close()

	// Create scanner for line-by-line processing
	scanner := bufio.NewScanner(file)

	// Process each line
	for scanner.Scan() {
		line := scanner.Text()

		// Parse the entry
		var entry WALEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("failed to unmarshal WAL entry: %w", err)
		}

		// Process the entry based on its type
		switch entry.Type {
		case "score":
			// Convert the data to a Score
			scoreData, err := json.Marshal(entry.Data)
			if err != nil {
				return fmt.Errorf("failed to marshal score data: %w", err)
			}

			var score models.Score
			if err := json.Unmarshal(scoreData, &score); err != nil {
				return fmt.Errorf("failed to unmarshal score: %w", err)
			}

			// Add the score to the store
			store.AddScore(score)

		case "snapshot":
			// Reset the store for a new snapshot
			store.mu.Lock()
			store.leaderboards = make(map[int64]*GameLeaderboard)
			store.mu.Unlock()

			// Convert the data to a map of scores
			snapshotData, err := json.Marshal(entry.Data)
			if err != nil {
				return fmt.Errorf("failed to marshal snapshot data: %w", err)
			}

			var snapshot map[int64][]models.Score
			if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
				return fmt.Errorf("failed to unmarshal snapshot: %w", err)
			}

			// Add all scores to the store
			for _, scores := range snapshot {
				for _, score := range scores {
					store.AddScore(score)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}
