package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/ringg-play/leaderboard-realtime/config"
	"github.com/ringg-play/leaderboard-realtime/models"
)

// PostgresRepository handles database operations
type PostgresRepository struct {
	db *sql.DB
}

// PostgresRepositoryInterface defines the interface for the PostgreSQL repository
type PostgresRepositoryInterface interface {
	SaveScore(score models.Score) error
	GetTopLeaders(gameID int64, limit int, window models.TimeWindow) ([]models.LeaderboardEntry, error)
	GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, error)
}

// CreatePool creates a new connection pool to the PostgreSQL database
func CreatePool(cfg *config.AppConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	// Create the required tables if they don't exist
	if err := initTables(db); err != nil {
		return nil, err
	}

	return &PostgresRepository{db: db}, nil
}

// initTables creates the required tables if they don't exist
func initTables(db *sql.DB) error {
	// Create the scores table
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS scores (
    id SERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    score BIGINT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL
);
`)
	if err != nil {
		return err
	}

	// Create indices separately (PostgreSQL syntax)
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_scores_game_user ON scores (game_id, user_id);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_scores_game_score ON scores (game_id, score DESC);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_scores_timestamp ON scores (timestamp);`)
	if err != nil {
		return err
	}

	return nil
}

// SaveScore saves a score to the database
func (r *PostgresRepository) SaveScore(score models.Score) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
INSERT INTO scores (game_id, user_id, score, timestamp)
VALUES ($1, $2, $3, $4)
`, score.GameID, score.UserID, score.Score, score.Timestamp)

	return err
}

// GetTopLeaders gets the top leaders for a game
func (r *PostgresRepository) GetTopLeaders(gameID int64, limit int, window models.TimeWindow) ([]models.LeaderboardEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
SELECT user_id, score, rank
FROM (
    SELECT
        user_id,
        score,
        RANK() OVER (ORDER BY score DESC) as rank
    FROM (
        SELECT DISTINCT ON (user_id) user_id, score
        FROM scores
        WHERE game_id = $1
    `

	args := []interface{}{gameID}
	argIndex := 2

	// Add timestamp filter if needed
	if start, end := window.GetTimeRange(); start != nil {
		query += fmt.Sprintf(" AND timestamp BETWEEN $%d AND $%d ", argIndex, argIndex+1)
		args = append(args, *start, end)
		argIndex += 2
	}

	query += `
        ORDER BY user_id, score DESC
    ) AS best_scores
) ranked_scores
WHERE rank <= $` + fmt.Sprintf("%d", argIndex)

	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LeaderboardEntry
	for rows.Next() {
		var entry models.LeaderboardEntry
		if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// GetPlayerRank gets a player's rank and percentile for a game
func (r *PostgresRepository) GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First, get the player's best score
	var score uint64
	scoreQuery := `
SELECT MAX(score) as score
FROM scores
WHERE game_id = $1 AND user_id = $2
`
	args := []interface{}{gameID, userID}
	argIndex := 3

	// Add timestamp filter if needed
	if start, end := window.GetTimeRange(); start != nil {
		scoreQuery += fmt.Sprintf(" AND timestamp BETWEEN $%d AND $%d ", argIndex, argIndex+1)
		args = append(args, *start, end)
		argIndex += 2
	}

	err := r.db.QueryRowContext(ctx, scoreQuery, args...).Scan(&score)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, fmt.Errorf("player not found")
	}
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Now get the rank and total players
	rankQuery := `
WITH player_scores AS (
    SELECT DISTINCT ON (user_id) user_id, score
    FROM scores
    WHERE game_id = $1
`
	rankArgs := []interface{}{gameID}
	rankArgIndex := 2

	// Add timestamp filter if needed
	if start, end := window.GetTimeRange(); start != nil {
		rankQuery += fmt.Sprintf(" AND timestamp BETWEEN $%d AND $%d ", rankArgIndex, rankArgIndex+1)
		rankArgs = append(rankArgs, *start, end)
		rankArgIndex += 2
	}

	rankQuery += `
    ORDER BY user_id, score DESC
)
SELECT
    (SELECT COUNT(*) FROM player_scores WHERE score > $` + fmt.Sprintf("%d", rankArgIndex) + `) + 1 AS rank,
    (SELECT COUNT(*) FROM player_scores) AS total
`

	rankArgs = append(rankArgs, score)

	var rank, total uint64
	err = r.db.QueryRowContext(ctx, rankQuery, rankArgs...).Scan(&rank, &total)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Calculate percentile
	var percentile float64
	if total > 0 {
		percentile = 100.0 * float64(total-rank) / float64(total)
	}

	return rank, percentile, score, total, nil
}
