package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/internal/db"
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
// @Example     {"game_id": 1, "user_id": 1, "score": 100, "timestamp": "2021-01-01T00:00:00Z"}
func GetTopLeadersHandler(store *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse game ID from path
		gameIDStr := c.Param("gameId")
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID"})
			return
		}

		// Parse limit from query
		limitStr := c.DefaultQuery("limit", "10")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
			return
		}

		// Parse time window from query
		windowStr := c.DefaultQuery("window", "")
		window := models.FromQueryParam(windowStr)

		// Get top leaders
		leaders := store.GetTopLeaders(gameID, limit, window)
		// totalPlayers := store.TotalPlayers(gameID)

		// Return response
		c.JSON(http.StatusOK, models.TopLeadersResponse{
			GameID:  gameID,
			Leaders: leaders,
			// TotalPlayers: totalPlayers,
			Window: window.Display,
		})
	}
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
func GetPlayerRankHandler(store *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse game ID from path
		gameIDStr := c.Param("gameId")
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID"})
			return
		}

		// Parse user ID from path
		userIDStr := c.Param("userId")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Parse time window from query
		windowStr := c.DefaultQuery("window", "")
		window := models.FromQueryParam(windowStr)

		// Get player rank
		rank, percentile, score, total, exists := store.GetPlayerRank(gameID, userID, window)
		if !exists {
			// c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
			c.JSON(http.StatusOK, gin.H{"error": "Player not found"})
			return
		}

		// Return response
		c.JSON(http.StatusOK, models.PlayerRankResponse{
			GameID:       gameID,
			UserID:       userID,
			Score:        score,
			Rank:         rank,
			Percentile:   percentile,
			TotalPlayers: total,
			Window:       window.Display,
		})
	}
}

// SubmitScoreHandler returns a handler for submitting a score
// @Summary      Submit a player's score
// @Description  Records a new score for a player in a game
// @Tags         leaderboard
// @Accept       json
// @Produce      json
// @Param        gameId  path      int  true  "Game ID"
// @Param        score   body      models.Score  true  "Score data"
// @Success      200
// @Failure      400     {object}  map[string]string
// @Router       /api/leaderboard/score/{gameId} [post]
func SubmitScoreHandler(store *store.Store, pgRepo db.PostgresRepositoryInterface, producer *mq.KafkaProducer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse score from request body
		var score models.Score
		if err := c.ShouldBindJSON(&score); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid score data"})
			return
		}

		// Set timestamp if not provided
		if score.Timestamp.IsZero() {
			score.Timestamp = time.Now().UTC()
		}

		// Validate score
		if score.GameID <= 0 || score.UserID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID or user ID"})
			return
		}

		// Send score to Kafka
		if err := producer.SendScore(c.Request.Context(), score); err != nil {
			// Log error, but don't block the request
			log.Printf("Error sending score to Kafka: %v", err)

		}

		// Return success
		c.Status(http.StatusOK)
	}
}
