package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/api"
	"github.com/ringg-play/leaderboard-realtime/internal/db"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
	"github.com/stretchr/testify/assert"
)

func setupTestServer(t *testing.T) (*gin.Engine, *db.LeaderboardStore) {
	gin.SetMode(gin.TestMode)

	// Create temp data directory
	dataDir := "test_data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create test data directory: %v", err)
	}

	// Create WAL directory
	walDir := filepath.Join(dataDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		t.Fatalf("Failed to create WAL directory: %v", err)
	}

	// Initialize in-memory store
	store := db.NewLeaderboardStore()

	// Create a router
	router := gin.New()

	// Create a mock PostgreSQL repository
	pgRepo := &mockPgRepo{}

	// Configure routes
	api.ConfigureRoutes(router, store, pgRepo)

	return router, store
}

func cleanupTest(t *testing.T) {
	// Remove temp data directory
	if err := os.RemoveAll("test_data"); err != nil {
		t.Logf("Failed to clean up test data directory: %v", err)
	}
}

func TestFullScenario(t *testing.T) {
	router, _ := setupTestServer(t)
	defer cleanupTest(t)

	// 1. Submit scores for multiple users in multiple games
	games := []int64{1, 2}
	users := []int64{101, 102, 103, 104, 105}

	// Submit scores
	for _, gameID := range games {
		for i, userID := range users {
			score := models.Score{
				GameID:    gameID,
				UserID:    userID,
				Score:     uint64((i + 1) * 100),
				Timestamp: time.Now().UTC(),
			}

			scoreJSON, _ := json.Marshal(score)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/leaderboard/score", bytes.NewBuffer(scoreJSON))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		}
	}

	// 2. Get top leaders for game 1
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard/top/1?limit=3", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var topResponse models.TopLeadersResponse
	err := json.Unmarshal(w.Body.Bytes(), &topResponse)
	assert.NoError(t, err)

	assert.Equal(t, int64(1), topResponse.GameID)
	// assert.Equal(t, uint64(5), topResponse.TotalPlayers)
	assert.Equal(t, 3, len(topResponse.Leaders))

	// Verify the order (highest score first)
	assert.Equal(t, int64(105), topResponse.Leaders[0].UserID)
	assert.Equal(t, uint64(500), topResponse.Leaders[0].Score)
	assert.Equal(t, uint64(1), topResponse.Leaders[0].Rank)

	assert.Equal(t, int64(104), topResponse.Leaders[1].UserID)
	assert.Equal(t, uint64(400), topResponse.Leaders[1].Score)
	assert.Equal(t, uint64(2), topResponse.Leaders[1].Rank)

	// 3. Get player rank for a specific user in game 1
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/rank/1/103", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var rankResponse models.PlayerRankResponse
	err = json.Unmarshal(w.Body.Bytes(), &rankResponse)
	assert.NoError(t, err)

	assert.Equal(t, int64(1), rankResponse.GameID)
	assert.Equal(t, int64(103), rankResponse.UserID)
	assert.Equal(t, uint64(300), rankResponse.Score)
	assert.Equal(t, uint64(3), rankResponse.Rank)
	assert.Equal(t, uint64(5), rankResponse.TotalPlayers)
	assert.InDelta(t, 40.0, rankResponse.Percentile, 0.1) // (5-3)/5 * 100 = 40%

	// 4. Submit a higher score for an existing user
	updatedScore := models.Score{
		GameID:    1,
		UserID:    101,
		Score:     600, // Higher than any previous score
		Timestamp: time.Now().UTC(),
	}

	scoreJSON, _ := json.Marshal(updatedScore)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/leaderboard/score", bytes.NewBuffer(scoreJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 5. Verify the updated leaderboard
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/top/1?limit=2", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updatedTopResponse models.TopLeadersResponse
	err = json.Unmarshal(w.Body.Bytes(), &updatedTopResponse)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(updatedTopResponse.Leaders))
	assert.Equal(t, int64(101), updatedTopResponse.Leaders[0].UserID) // User 101 should be first now
	assert.Equal(t, uint64(600), updatedTopResponse.Leaders[0].Score)
	assert.Equal(t, int64(105), updatedTopResponse.Leaders[1].UserID) // User 105 should be second

	// 6. Test time window functionality
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/top/1?limit=2&window=3d", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var windowResponse models.TopLeadersResponse
	err = json.Unmarshal(w.Body.Bytes(), &windowResponse)
	assert.NoError(t, err)

	assert.Equal(t, "3d", windowResponse.Window)

	// 7. Check health endpoint
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var healthResponse models.HealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &healthResponse)
	assert.NoError(t, err)
	assert.Equal(t, "OK", healthResponse.Status)
}
