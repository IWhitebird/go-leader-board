package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ringg-play/leaderboard-realtime/config"
	"github.com/ringg-play/leaderboard-realtime/internal/models"
	"github.com/segmentio/kafka-go"
)

// KafkaProducer handles score message production to Kafka with high performance
type KafkaProducer struct {
	writer        *kafka.Writer
	connected     bool
	scoreChan     chan models.Score
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	batchSize     int
	flushInterval time.Duration
	mu            sync.RWMutex
}

// NewKafkaProducer creates a new high-performance Kafka producer
func NewKafkaProducer(cfg *config.AppConfig) (*KafkaProducer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create single Kafka writer
	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Topic:    cfg.Kafka.ScoresTopicPrefix,
		Balancer: &kafka.Hash{}, // Hash balancer for distribution

		// High-performance settings
		BatchSize:    500,                   // Batch up to 500 messages
		BatchBytes:   1024 * 1024,           // Batch up to 1MB
		BatchTimeout: 50 * time.Millisecond, // Force flush every 10ms

		// Reliability settings
		RequiredAcks: kafka.RequireOne, // Only require leader acknowledgment for speed
		Async:        true,             // Enable async writes

		// Connection settings
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,

		// Compression for better throughput
		Compression: kafka.Snappy,

		// Buffer settings
		MaxAttempts: 3,
	}

	producer := &KafkaProducer{
		writer:        writer,
		connected:     false,
		scoreChan:     make(chan models.Score, 2000), // Large buffer for high throughput
		ctx:           ctx,
		cancel:        cancel,
		batchSize:     500,                   // Batch 500 messages at a time
		flushInterval: 10 * time.Millisecond, // Flush every 10ms for low latency
	}

	// Test connection to Kafka
	maxRetries := 5
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = producer.testConnection(cfg.Kafka.Brokers); err == nil {
			break
		}
		log.Printf("Failed to connect to Kafka (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Kafka after %d attempts: %v", maxRetries, err)
	}

	producer.connected = true

	// Start the async batch processor
	producer.startBatchProcessor()

	return producer, nil
}

// testConnection tests the connection to Kafka brokers
func (p *KafkaProducer) testConnection(brokers []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	log.Printf("Successfully connected to Kafka cluster")
	return nil
}

// startBatchProcessor starts the async batch processing goroutine
func (p *KafkaProducer) startBatchProcessor() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		batch := make([]models.Score, 0, p.batchSize)
		ticker := time.NewTicker(p.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case score := <-p.scoreChan:
				batch = append(batch, score)

				// Flush when batch is full
				if len(batch) >= p.batchSize {
					p.flushBatch(batch)
					batch = batch[:0] // Reset slice but keep capacity
				}

			case <-ticker.C:
				// Flush on timer if we have any pending messages
				if len(batch) > 0 {
					p.flushBatch(batch)
					batch = batch[:0]
				}

			case <-p.ctx.Done():
				// Final flush before shutdown
				if len(batch) > 0 {
					p.flushBatch(batch)
				}
				return
			}
		}
	}()
}

// flushBatch sends a batch of scores to Kafka
func (p *KafkaProducer) flushBatch(scores []models.Score) {
	if len(scores) == 0 {
		return
	}

	// Create Kafka messages
	messages := make([]kafka.Message, len(scores))
	for i, score := range scores {
		scoreJSON, err := json.Marshal(score)
		if err != nil {
			log.Printf("Error marshaling score: %v", err)
			continue
		}

		// Use game_id as key for partitioning - same game goes to same partition
		messages[i] = kafka.Message{
			Key:   []byte(fmt.Sprintf("game-%d", score.GameID)),
			Value: scoreJSON,
			Time:  time.Now(),
		}
	}

	// Send batch with timeout context
	ctx, cancel := context.WithTimeout(p.ctx, 15*time.Second)
	defer cancel()

	start := time.Now()
	err := p.writer.WriteMessages(ctx, messages...)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Error sending batch of %d scores to Kafka (took %v): %v", len(messages), duration, err)
	} else {
		log.Printf("Successfully sent batch of %d scores to Kafka (took %v)", len(messages), duration)
	}
}

// SendScore sends a score to Kafka asynchronously
func (p *KafkaProducer) SendScore(ctx context.Context, score models.Score) error {
	p.mu.RLock()
	connected := p.connected
	p.mu.RUnlock()

	if !connected {
		return fmt.Errorf("producer not connected")
	}

	// Non-blocking send to channel
	select {
	case p.scoreChan <- score:
		return nil
	default:
		// Channel is full - this indicates we're overwhelmed
		return fmt.Errorf("producer queue full - too many concurrent writes")
	}
}

// Close gracefully shuts down the Kafka producer
func (p *KafkaProducer) Close() error {
	log.Printf("Shutting down Kafka producer...")

	// Stop accepting new messages
	p.mu.Lock()
	p.connected = false
	p.mu.Unlock()

	// Cancel context to stop background goroutines
	p.cancel()

	// Wait for batch processor to finish
	p.wg.Wait()

	// Close the writer
	if p.writer != nil {
		err := p.writer.Close()
		log.Printf("Kafka producer shutdown complete")
		return err
	}

	return nil
}
