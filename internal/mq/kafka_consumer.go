package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ringg-play/leaderboard-realtime/config"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
	"github.com/ringg-play/leaderboard-realtime/internal/store"
	"github.com/segmentio/kafka-go"
)

// KafkaConsumer handles consuming and processing score messages from Kafka
type KafkaConsumer struct {
	reader        *kafka.Reader
	store         *store.Store
	batchSize     int
	timeout       time.Duration
	brokers       []string
	topic         string
	consumerGroup string
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(cfg *config.AppConfig, store *store.Store) (*KafkaConsumer, error) {
	consumer := &KafkaConsumer{
		store:         store,
		batchSize:     cfg.Kafka.BatchSize,
		timeout:       time.Duration(cfg.Kafka.BatchTimeout) * time.Second,
		brokers:       cfg.Kafka.Brokers,
		topic:         cfg.Kafka.ScoresTopic,
		consumerGroup: cfg.Kafka.ConsumerGroup,
	}

	// Retry connecting to Kafka
	maxRetries := 5
	var err error
	for i := range maxRetries {
		if err = consumer.connect(); err == nil {
			break
		}
		log.Printf("Failed to connect consumer to Kafka (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect consumer to Kafka after %d attempts: %v", maxRetries, err)
	}

	return consumer, nil
}

// connect establishes connection to Kafka
func (c *KafkaConsumer) connect() error {
	// Verify topic exists by connecting to a broker
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", c.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	// List topics to check if our topic exists
	topics, err := conn.ReadPartitions()
	if err != nil {
		return fmt.Errorf("failed to read topics: %v", err)
	}

	topicExists := false
	for _, t := range topics {
		if t.Topic == c.topic {
			topicExists = true
			break
		}
	}

	if !topicExists {
		log.Printf("Warning: Topic %s does not exist, consumer may not function correctly", c.topic)
	}

	// Create Kafka reader with consumer group configuration
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:         c.brokers,
		Topic:           c.topic,
		GroupID:         c.consumerGroup,
		MinBytes:        10e3, // 10KB
		MaxBytes:        10e6, // 10MB
		CommitInterval:  time.Second,
		ReadLagInterval: time.Second * 5,
		MaxWait:         time.Second * 3,
		StartOffset:     kafka.FirstOffset,
		SessionTimeout:  time.Second * 10,
	})

	c.reader = reader
	return nil
}

// StartConsumer starts the consumer to process score messages
func (c *KafkaConsumer) StartConsumer(ctx context.Context) {
	log.Println("Starting Kafka consumer for scores")

	go func() {
		defer c.reader.Close()

		// Process messages in batches
		for {
			select {
			case <-ctx.Done():
				log.Println("Kafka consumer shutting down")
				return
			default:
				if err := c.processBatch(ctx); err != nil {
					log.Printf("Error processing batch: %v", err)
					// Add a small delay before retrying to avoid tight loops on persistent errors
					time.Sleep(time.Second * 2)
				}
			}
		}
	}()
}

// processBatch processes a batch of score messages
func (c *KafkaConsumer) processBatch(ctx context.Context) error {
	batch := make([]models.Score, 0, c.batchSize)
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()

	// Create a context with timeout for batch processing
	batchCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Collect messages until batch size is reached or timeout occurs
	for len(batch) < c.batchSize {
		select {
		case <-timer.C:
			// Timeout reached, process the batch even if not full
			if len(batch) > 0 {
				return c.saveBatchToPostgres(batch)
			}
			return nil
		case <-ctx.Done():
			// Context canceled, save any pending messages and exit
			if len(batch) > 0 {
				return c.saveBatchToPostgres(batch)
			}
			return ctx.Err()
		default:
			// Try to fetch a message with a short timeout
			fetchCtx, fetchCancel := context.WithTimeout(batchCtx, 100*time.Millisecond)
			message, err := c.reader.FetchMessage(fetchCtx)
			fetchCancel()

			if err != nil {
				// If timeout, continue to check other conditions
				if err == context.DeadlineExceeded {
					continue
				}
				return fmt.Errorf("error fetching message from Kafka: %v", err)
			}

			// Parse the score
			var score models.Score
			if err := json.Unmarshal(message.Value, &score); err != nil {
				log.Printf("Error unmarshaling score: %v", err)
				// Commit the invalid message to avoid getting stuck
				if commitErr := c.reader.CommitMessages(ctx, message); commitErr != nil {
					log.Printf("Error committing invalid message: %v", commitErr)
				}
				continue
			}

			// Add to batch
			batch = append(batch, score)

			// Commit the message
			if err := c.reader.CommitMessages(ctx, message); err != nil {
				return fmt.Errorf("error committing message: %v", err)
			}
		}
	}

	// Process full batch
	if len(batch) > 0 {
		return c.saveBatchToPostgres(batch)
	}

	return nil
}

// saveBatchToPostgres saves a batch of scores to PostgreSQL
func (c *KafkaConsumer) saveBatchToPostgres(batch []models.Score) error {
	if len(batch) == 0 {
		return nil
	}

	log.Printf("Saving batch of %d scores to PostgreSQL", len(batch))

	// Use the batch insert method for better performance
	if err := c.store.SaveScoreBatch(batch); err != nil {
		log.Printf("Error saving batch to PostgreSQL: %v", err)

		// Fall back to individual inserts if batch fails
		// var failedCount int
		// for _, score := range batch {
		// 	if err := c.store.SaveScore(score); err != nil {
		// 		failedCount++
		// 		log.Printf("Error saving individual score to PostgreSQL: %v", err)
		// 	}
		// }

		// if failedCount > 0 {
		// 	return fmt.Errorf("failed to save %d/%d scores", failedCount, len(batch))
		// }
	}

	return nil
}

// Close closes the Kafka consumer
func (c *KafkaConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
