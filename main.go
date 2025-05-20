package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ringg-play/leaderboard-realtime/api"
	"github.com/ringg-play/leaderboard-realtime/config"
	"github.com/ringg-play/leaderboard-realtime/db"
	_ "github.com/ringg-play/leaderboard-realtime/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// Initialize logger
	log.Println("Starting leaderboard service")

	// Load configuration
	cfg := config.NewAppConfig()
	log.Printf("Configuration: %+v", cfg)

	// Setup data directory
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create WAL directory
	walDir := filepath.Join(dataDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		log.Fatalf("Failed to create WAL directory: %v", err)
	}

	// Initialize in-memory store with WAL for persistence
	log.Println("Initializing in-memory store with WAL persistence")
	store, err := db.NewLeaderboardStoreWithWAL(walDir)
	if err != nil {
		log.Fatalf("Failed to initialize store with WAL: %v", err)
	}
	defer store.Close()

	// Initialize PostgreSQL connection and repository
	log.Println("Initializing PostgreSQL connection")
	pgPool, err := db.CreatePool(cfg)
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL pool: %v", err)
	}
	defer pgPool.Close()

	log.Println("Initializing PostgreSQL repository")
	pgRepo, err := db.NewPostgresRepository(pgPool)
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL repository: %v", err)
	}
	log.Println("PostgreSQL connection established")

	// Configure router using Gin
	router := gin.Default()
	// Mount middleware and routes
	api.ConfigureRoutes(router, store, pgRepo)

	// Swagger documentation routes
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// Wait for interrupt signal to gracefully shut down the server
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		log.Println("Shutdown signal received, stopping server gracefully...")

		// Create a deadline for server shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		log.Println("Server gracefully stopped")
	}()

	// Start server in a goroutine
	log.Printf("Starting server on http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Head to http://%s:%d/swagger/index.html to see the API documentation", cfg.Server.Host, cfg.Server.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
