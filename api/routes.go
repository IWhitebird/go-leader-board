package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/db"
)

func ConfigureRoutes(r *gin.Engine, store *db.LeaderboardStore, pgRepo db.PostgresRepositoryInterface) {
	// API group
	api := r.Group("/api")

	// Health endpoint
	api.GET("/health", HealthHandler())

	// Leaderboard endpoints
	leaderboard := api.Group("/leaderboard")
	{
		// Get top leaders for a game
		leaderboard.GET("/top/:gameId", GetTopLeadersHandler(store))

		// Get a player's rank for a game
		leaderboard.GET("/rank/:gameId/:userId", GetPlayerRankHandler(store))

		// Submit a score
		leaderboard.POST("/score", SubmitScoreHandler(store, pgRepo))
	}
}
