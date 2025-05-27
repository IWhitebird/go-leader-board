package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IWhitebird/go-leader-board/api"
	"github.com/IWhitebird/go-leader-board/internal/models"
	"github.com/IWhitebird/go-leader-board/internal/store"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRouter() (*gin.Engine, *store.Store) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := store.NewStore(nil)
	responseCache := persistence.NewInMemoryStore(time.Minute)

	api.ConfigureRoutes(router, store, nil, nil, responseCache)

	return router, store
}

func TestGetTopLeadersHandler(t *testing.T) {
	router, store := setupRouter()

	// Add some test data
	now := time.Now().UTC()
	store.AddScore(models.Score{GameID: 1, UserID: 1, Score: 100, Timestamp: now})
	store.AddScore(models.Score{GameID: 1, UserID: 2, Score: 200, Timestamp: now})
	store.AddScore(models.Score{GameID: 1, UserID: 3, Score: 150, Timestamp: now})

	// Test valid request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard/top/1?limit=2", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.TopLeadersResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, int64(1), response.GameID)
	// assert.Equal(t, uint64(3), response.TotalPlayers)
	assert.Equal(t, 2, len(response.Leaders))
	assert.Equal(t, int64(2), response.Leaders[0].UserID)
	assert.Equal(t, uint64(200), response.Leaders[0].Score)

	// Test with time window
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/top/1?limit=2&window=24h", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var windowResponse models.TopLeadersResponse
	err = json.Unmarshal(w.Body.Bytes(), &windowResponse)
	assert.NoError(t, err)

	assert.Equal(t, "24h", windowResponse.Window)

	// Test invalid game ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/top/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test invalid limit
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/top/1?limit=-5", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPlayerRankHandler(t *testing.T) {
	router, store := setupRouter()

	// Add some test data
	now := time.Now().UTC()
	store.AddScore(models.Score{GameID: 1, UserID: 1, Score: 100, Timestamp: now})
	store.AddScore(models.Score{GameID: 1, UserID: 2, Score: 200, Timestamp: now})
	store.AddScore(models.Score{GameID: 1, UserID: 3, Score: 150, Timestamp: now})

	// Test valid request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/leaderboard/rank/1/2", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.PlayerRankResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, int64(1), response.GameID)
	assert.Equal(t, int64(2), response.UserID)
	assert.Equal(t, uint64(200), response.Score)
	assert.Equal(t, uint64(1), response.Rank)
	assert.InDelta(t, 66.67, response.Percentile, 0.1)

	// Test with time window
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/rank/1/2?window=24h", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var windowResponse models.PlayerRankResponse
	err = json.Unmarshal(w.Body.Bytes(), &windowResponse)
	assert.NoError(t, err)

	assert.Equal(t, "24h", windowResponse.Window)

	// Test player not found
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/rank/1/99", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test invalid game ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/rank/invalid/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test invalid user ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/leaderboard/rank/1/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubmitScoreHandler(t *testing.T) {
	router, _ := setupRouter()

	// Test valid request
	score := models.Score{
		GameID: 1,
		UserID: 1,
		Score:  100,
	}

	scoreJSON, _ := json.Marshal(score)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/leaderboard/score", bytes.NewBuffer(scoreJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Note: Score won't be immediately available in store since it goes through Kafka
	// In a real scenario, the Kafka consumer would process it and add to store
	// For testing immediate store updates, use store.AddScore() directly

	// Test invalid JSON
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/leaderboard/score", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test invalid score data (negative game ID)
	invalidScore := models.Score{
		GameID: -1,
		UserID: 1,
		Score:  100,
	}

	invalidScoreJSON, _ := json.Marshal(invalidScore)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/leaderboard/score", bytes.NewBuffer(invalidScoreJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
