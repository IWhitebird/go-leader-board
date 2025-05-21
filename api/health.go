package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
)

// HealthHandler returns a handler for the health endpoint
// @Summary      Health check endpoint
// @Description  Returns the current status of the API
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.HealthResponse
// @Router       /api/health [get]
func HealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response := models.HealthResponse{
			Status:    "OK",
			Version:   "1.0.0",
			Timestamp: time.Now().UTC(),
		}
		c.JSON(http.StatusOK, response)
	}
}
