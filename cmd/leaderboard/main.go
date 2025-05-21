package main

import (
	"context"
	"database/sql"
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
	"github.com/ringg-play/leaderboard-realtime/internal/db"
	"github.com/ringg-play/leaderboard-realtime/internal/mq"
	"github.com/ringg-play/leaderboard-realtime/internal/store"

	_ "github.com/ringg-play/leaderboard-realtime/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	log.Println("Starting leaderboard service")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.NewAppConfig()
	log.Printf("Configuration: %+v", cfg)

	_, walDir := setupDirectories()
	store := setupStore(walDir)
	defer store.Close()

	pgPool, pgRepo := setupPostgres(cfg)
	defer pgPool.Close()

	producer, consumer := setupKafka(cfg, pgRepo, ctx)
	defer producer.Close()
	defer consumer.Close()

	router := setupRouter(store, pgRepo, producer)
	server := setupServer(cfg, router)

	handleGracefulShutdown(server, cancel)
	startServer(cfg, server)
}

func setupDirectories() (string, string) {
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	walDir := filepath.Join(dataDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		log.Fatalf("Failed to create WAL directory: %v", err)
	}

	return dataDir, walDir
}

func setupStore(walDir string) *store.LeaderboardStore {
	log.Println("Initializing in-memory store with WAL persistence")
	store, err := store.NewLeaderboardStoreWithWAL(walDir)
	if err != nil {
		log.Fatalf("Failed to initialize store with WAL: %v", err)
	}
	return store
}

func setupPostgres(cfg *config.AppConfig) (*sql.DB, *db.PostgresRepository) {
	log.Println("Initializing PostgreSQL connection")
	pgPool, err := db.CreatePool(cfg)
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL pool: %v", err)
	}

	log.Println("Initializing PostgreSQL repository")
	pgRepo, err := db.NewPostgresRepository(pgPool)
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL repository: %v", err)
	}
	log.Println("PostgreSQL connection established")

	return pgPool, pgRepo
}

func setupKafka(cfg *config.AppConfig, pgRepo *db.PostgresRepository, ctx context.Context) (*mq.KafkaProducer, *mq.KafkaConsumer) {
	log.Println("Initializing Kafka producer")

	// Add retry logic for Kafka initialization
	var producer *mq.KafkaProducer
	var consumer *mq.KafkaConsumer
	var err error

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		producer, err = mq.NewKafkaProducer(cfg)
		if err == nil {
			break
		}
		log.Printf("Failed to initialize Kafka producer (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(2+i) * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer after %d attempts: %v", maxRetries, err)
	}
	log.Println("Kafka producer initialized")

	log.Println("Initializing Kafka consumer")
	for i := 0; i < maxRetries; i++ {
		consumer, err = mq.NewKafkaConsumer(cfg, pgRepo)
		if err == nil {
			break
		}
		log.Printf("Failed to initialize Kafka consumer (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(2+i) * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer after %d attempts: %v", maxRetries, err)
	}

	consumer.StartConsumer(ctx)
	log.Println("Kafka consumer started")

	return producer, consumer
}

func setupRouter(store *store.LeaderboardStore, pgRepo *db.PostgresRepository, producer *mq.KafkaProducer) *gin.Engine {
	router := gin.Default()
	api.ConfigureRoutes(router, store, pgRepo, producer)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return router
}

func setupServer(cfg *config.AppConfig, router *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}
}

func handleGracefulShutdown(server *http.Server, cancel context.CancelFunc) {
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		log.Println("Shutdown signal received, stopping server gracefully...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		log.Println("Server gracefully stopped")
	}()
}

func startServer(cfg *config.AppConfig, server *http.Server) {
	log.Printf("Starting server on http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Head to http://%s:%d/swagger/index.html to see the API documentation", cfg.Server.Host, cfg.Server.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
