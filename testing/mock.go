package test

import "github.com/IWhitebird/go-leader-board/internal/models"

// Mock PostgreSQL repository for testing
type mockPgRepo struct{}

func (m *mockPgRepo) SaveScore(score models.Score) error {
	return nil
}

func (m *mockPgRepo) GetTopLeaders(gameID int64, limit int, window models.TimeWindow) ([]models.LeaderboardEntry, error) {
	return nil, nil
}

func (m *mockPgRepo) GetPlayerRank(gameID, userID int64, window models.TimeWindow) (uint64, float64, uint64, uint64, error) {
	return 0, 0, 0, 0, nil
}
