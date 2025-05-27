package api

import (
	"net/http"
	"strconv"
	"time"

	responseCache "github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/internal/db"
	"github.com/ringg-play/leaderboard-realtime/internal/logging"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
	"github.com/ringg-play/leaderboard-realtime/internal/mq"
	"github.com/ringg-play/leaderboard-realtime/internal/store"
)

// GetTopLeadersHandler returns a handler for getting top leaders
// @Summary      Get top leaders for a game
// @Description  Returns the top scoring players for a specific game
// @Tags         leaderboard
// @Accept       json
// @Produce      json
// @Param        gameId  path      int  true  "Game ID"
// @Param        limit   query     int  false  "Number of leaders to return" default(10)
// @Param        window  query     string  false  "Time window (empty for all-time, 24h for last 24 hours, 3d for 3 days, 7d for 7 days)" Enums(24h,3d,7d)
// @Success      200     {object}  models.TopLeadersResponse
// @Failure      400     {object}  map[string]string
// @Router       /api/leaderboard/top/{gameId} [get]
func GetTopLeadersHandler(store *store.Store, responseCacheStore *persistence.InMemoryStore) gin.HandlerFunc {
	return responseCache.CachePage(responseCacheStore, time.Second*5, func(c *gin.Context) {
		gameIDStr := c.Param("gameId")
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID"})
			return
		}

		limitStr := c.DefaultQuery("limit", "10")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
			return
		}

		windowStr := c.DefaultQuery("window", "")
		window, err := models.FromQueryParam(windowStr)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid window"})
			return
		}

		leaders := store.GetTopLeaders(gameID, limit, window)
		totalPlayers := store.TotalPlayers(gameID)

		c.JSON(http.StatusOK, models.TopLeadersResponse{
			GameID:       gameID,
			Leaders:      leaders,
			TotalPlayers: totalPlayers,
			Window:       window.Display,
		})
	})
}

// GetPlayerRankHandler returns a handler for getting a player's rank
// @Summary      Get a player's rank
// @Description  Returns the rank and percentile for a specific player in a game
// @Tags         leaderboard
// @Accept       json
// @Produce      json
// @Param        gameId  path      int  true  "Game ID"
// @Param        userId  path      int  true  "User ID"
// @Param        window  query     string  false  "Time window (empty for all-time, 24h for last 24 hours, 3d for 3 days, 7d for 7 days)" Enums(24h,3d,7d)
// @Success      200     {object}  models.PlayerRankResponse
// @Failure      400     {object}  map[string]string
// @Failure      404     {object}  map[string]string
// @Router       /api/leaderboard/rank/{gameId}/{userId} [get]
func GetPlayerRankHandler(store *store.Store, responseCacheStore *persistence.InMemoryStore) gin.HandlerFunc {
	return responseCache.CachePage(responseCacheStore, time.Second*5, func(c *gin.Context) {
		gameIDStr := c.Param("gameId")
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID"})
			return
		}

		userIDStr := c.Param("userId")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		windowStr := c.DefaultQuery("window", "")
		window, err := models.FromQueryParam(windowStr)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid window"})
			return
		}

		rank, percentile, score, total, exists := store.GetPlayerRank(gameID, userID, window)
		if !exists {
			c.JSON(http.StatusOK, gin.H{"error": "Player not found"})
			return
		}

		c.JSON(http.StatusOK, models.PlayerRankResponse{
			GameID:       gameID,
			UserID:       userID,
			Score:        score,
			Rank:         rank,
			Percentile:   percentile,
			TotalPlayers: total,
			Window:       window.Display,
		})
	})
}

// SubmitScoreHandler returns a handler for submitting a score
// @Summary      Submit a player's score
// @Description  Records a new score for a player in a game
// @Tags         leaderboard
// @Accept       json
// @Produce      json
// @Param        score   body      models.Score  true  "Score data"
// @Success      200
// @Failure      400     {object}  map[string]string
// @Router       /api/leaderboard/score [post]
func SubmitScoreHandler(store *store.Store, pgRepo db.PostgresRepositoryInterface, producer *mq.KafkaProducer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var score models.Score
		if err := c.ShouldBindJSON(&score); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid score data"})
			return
		}

		if score.Timestamp.IsZero() {
			score.Timestamp = time.Now().UTC()
		}

		if score.GameID <= 0 || score.UserID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID or user ID"})
			return
		}

		if err := store.AddScore(score); err != nil {
			logging.Error("Error adding score to store:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save score"})
			return
		}

		if producer != nil {
			if err := producer.SendScore(c.Request.Context(), score); err != nil {
				logging.Error("Error sending score to Kafka:", err)
			}
		}

		c.Status(http.StatusOK)
	}
}
