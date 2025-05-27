package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/ringg-play/leaderboard-realtime/config"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
)

//go:embed sql/init.sql
var initSQL string

type PostgresRepository struct {
	db *sql.DB
}

type PostgresRepositoryInterface interface {
	SaveScore(score models.Score) error
	GetTopLeaders(gameID int64, limit int, window models.TimeWindow) ([]models.LeaderboardEntry, error)
	GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, error)
	SaveScoreBatch(scores []models.Score) error
	GetAllScores() ([]models.Score, error)
	GetAllScoresForGame(gameID int64) ([]models.Score, error)
}

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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	if err := initTables(db); err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func initTables(db *sql.DB) error {
	_, err := db.Exec(initSQL)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) SaveScore(score models.Score) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
INSERT INTO scores (game_id, user_id, score, timestamp)
VALUES ($1, $2, $3, $4)
`, score.GameID, score.UserID, score.Score, score.Timestamp)

	return err
}

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

	args := []any{gameID}
	argIndex := 2

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

func (r *PostgresRepository) GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var score uint64
	scoreQuery := `
SELECT MAX(score) as score
FROM scores
WHERE game_id = $1 AND user_id = $2
`
	args := []any{gameID, userID}
	argIndex := 3

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

	rankQuery := `
WITH player_scores AS (
    SELECT DISTINCT ON (user_id) user_id, score
    FROM scores
    WHERE game_id = $1
`
	rankArgs := []any{gameID}
	rankArgIndex := 2

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

	var percentile float64
	if total > 0 {
		percentile = 100.0 * float64(total-rank) / float64(total)
	}

	return rank, percentile, score, total, nil
}

func (r *PostgresRepository) SaveScoreBatch(scores []models.Score) error {
	if len(scores) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO scores (game_id, user_id, score, timestamp)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, score := range scores {
		_, err = stmt.ExecContext(ctx, score.GameID, score.UserID, score.Score, score.Timestamp)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresRepository) GetAllGames() ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	query := `
SELECT DISTINCT game_id
FROM scores
ORDER BY game_id
`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []int64
	for rows.Next() {
		var game int64
		if err := rows.Scan(&game); err != nil {
			return nil, err
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return games, nil
}

func (r *PostgresRepository) GetAllScores() ([]models.Score, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := `
SELECT game_id, user_id, score, timestamp
FROM scores
ORDER BY game_id, timestamp DESC
`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []models.Score
	for rows.Next() {
		var score models.Score
		if err := rows.Scan(&score.GameID, &score.UserID, &score.Score, &score.Timestamp); err != nil {
			return nil, err
		}
		scores = append(scores, score)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return scores, nil
}

func (r *PostgresRepository) GetAllScoresForGame(gameID int64) ([]models.Score, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	query := `
SELECT game_id, user_id, score, timestamp
FROM scores
WHERE game_id = $1
ORDER BY timestamp DESC
`

	rows, err := r.db.QueryContext(ctx, query, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []models.Score
	for rows.Next() {
		var score models.Score
		if err := rows.Scan(&score.GameID, &score.UserID, &score.Score, &score.Timestamp); err != nil {
			return nil, err
		}
		scores = append(scores, score)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return scores, nil
}
