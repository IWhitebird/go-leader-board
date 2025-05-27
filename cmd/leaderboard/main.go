package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IWhitebird/go-leader-board/api"
	"github.com/IWhitebird/go-leader-board/config"
	"github.com/IWhitebird/go-leader-board/internal/db"
	"github.com/IWhitebird/go-leader-board/internal/logging"
	"github.com/IWhitebird/go-leader-board/internal/mq"
	"github.com/IWhitebird/go-leader-board/internal/store"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"

	_ "github.com/IWhitebird/go-leader-board/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	log.Println("Starting leaderboard service")

	//Initialize context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Initialize configuration
	cfg := config.NewAppConfig()

	//Initialize logging
	logging.Init()

	//Initialize postgres
	pgPool, pgRepo := setupPostgres(cfg)
	defer pgPool.Close()

	//Initialize in-memory store
	store := setupStore(pgRepo, cfg)
	defer store.Close()

	//Initialize kafka
	producer, consumer := setupKafka(cfg, store, ctx)
	defer producer.Close()
	defer consumer.Close()

	//Initialize router
	router := setupRouter(store, pgRepo, producer)
	server := setupServer(cfg, router)

	//Start server
	handleGracefulShutdown(server, cancel)
	startServer(cfg, server)
}

func setupStore(db *db.PostgresRepository, cfg *config.AppConfig) *store.Store {
	log.Println("Initializing in-memory store")
	store := store.NewStore(db)

	// Initialize the store from PostgreSQL database
	log.Println("Loading existing data from PostgreSQL...")
	if err := store.InitializeFromDatabase(cfg); err != nil {
		log.Fatalf("Failed to initialize store from database: %v", err)
	}
	log.Println("Store initialization completed successfully")

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

func setupKafka(cfg *config.AppConfig, store *store.Store, ctx context.Context) (*mq.KafkaProducer, *mq.KafkaConsumer) {
	log.Println("Initializing Kafka producer")

	// Add retry logic for Kafka initialization
	var producer *mq.KafkaProducer
	var consumer *mq.KafkaConsumer
	var err error

	maxRetries := 5
	for i := range maxRetries {
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
	for i := range maxRetries {
		consumer, err = mq.NewKafkaConsumer(cfg, store)
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

func setupRouter(store *store.Store, pgRepo *db.PostgresRepository, producer *mq.KafkaProducer) *gin.Engine {
	router := gin.Default()
	responseCache := persistence.NewInMemoryStore(time.Second)
	api.ConfigureRoutes(router, store, pgRepo, producer, responseCache)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return router
}

func setupServer(cfg *config.AppConfig, router *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}
}

func startServer(cfg *config.AppConfig, server *http.Server) {
	log.Printf("Starting server on http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Head to http://%s:%d/swagger/index.html to see the API documentation", cfg.Server.Host, cfg.Server.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
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
