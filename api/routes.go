package api

import (
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/internal/db"
	"github.com/ringg-play/leaderboard-realtime/internal/mq"
	"github.com/ringg-play/leaderboard-realtime/internal/store"
)

func ConfigureRoutes(
	r *gin.Engine,
	store *store.Store,
	pgRepo db.PostgresRepositoryInterface,
	producer *mq.KafkaProducer,
	responseCache *persistence.InMemoryStore) {
	// API group
	api := r.Group("/api")

	// Health endpoint
	api.GET("/health", HealthHandler())

	// Leaderboard endpoints
	leaderboard := api.Group("/leaderboard")
	{
		// Get top leaders for a game
		leaderboard.GET("/top/:gameId", GetTopLeadersHandler(store, responseCache))

		// Get a player's rank for a game
		leaderboard.GET("/rank/:gameId/:userId", GetPlayerRankHandler(store, responseCache))

		// Submit a score
		leaderboard.POST("/score", SubmitScoreHandler(store, pgRepo, producer))
	}
}
