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
	topic         string
	connected     bool
	brokers       []string
	scoreChan     chan models.Score
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	batchSize     int
	flushInterval time.Duration
	mu            sync.RWMutex
	metrics       *ProducerMetrics
}

// ProducerMetrics tracks producer performance
type ProducerMetrics struct {
	TotalSent     int64
	TotalErrors   int64
	BatchesSent   int64
	LastFlushTime time.Time
	mu            sync.RWMutex
}

// NewKafkaProducer creates a new high-performance Kafka producer
func NewKafkaProducer(cfg *config.AppConfig) (*KafkaProducer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	producer := &KafkaProducer{
		topic:         cfg.Kafka.ScoresTopic,
		connected:     false,
		brokers:       cfg.Kafka.Brokers,
		scoreChan:     make(chan models.Score, 10000), // Large buffer for high throughput
		ctx:           ctx,
		cancel:        cancel,
		batchSize:     500,                   // Batch 500 messages at a time
		flushInterval: 10 * time.Millisecond, // Flush every 10ms for low latency
		metrics:       &ProducerMetrics{},
	}

	// Retry logic for connecting to Kafka
	maxRetries := 5
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = producer.connect(); err == nil {
			break
		}
		log.Printf("Failed to connect to Kafka (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Kafka after %d attempts: %v", maxRetries, err)
	}

	// Ensure topic exists
	if err := producer.ensureTopicExists(cfg.Kafka.ScoresTopic); err != nil {
		log.Printf("Warning: could not verify topic exists: %v", err)
	}

	// Start the async batch processor
	producer.startBatchProcessor()

	// Start metrics logger
	go producer.logMetrics()

	return producer, nil
}

// connect establishes connection to Kafka with high-performance settings
func (p *KafkaProducer) connect() error {
	// Create Kafka writer with optimized settings for high throughput
	writer := &kafka.Writer{
		Addr:     kafka.TCP(p.brokers...),
		Topic:    p.topic,
		Balancer: &kafka.Hash{}, // Hash balancer for better distribution

		// High-performance settings
		BatchSize:    500,                   // Batch up to 500 messages
		BatchBytes:   1024 * 1024,           // Batch up to 1MB
		BatchTimeout: 10 * time.Millisecond, // Force flush every 10ms

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

	p.writer = writer
	p.connected = true

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", p.brokers[0])
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
			p.updateMetrics(0, 1, 0)
			continue
		}

		messages[i] = kafka.Message{
			Key:   fmt.Appendf(nil, "%d-%d", score.GameID, score.UserID),
			Value: scoreJSON,
			Time:  time.Now(),
		}
	}

	// Send batch with timeout context (not dependent on HTTP request)
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	start := time.Now()
	err := p.writer.WriteMessages(ctx, messages...)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Error sending batch of %d scores to Kafka (took %v): %v", len(messages), duration, err)
		p.updateMetrics(0, int64(len(messages)), 0)
	} else {
		log.Printf("Successfully sent batch of %d scores to Kafka (took %v)", len(messages), duration)
		p.updateMetrics(int64(len(messages)), 0, 1)
	}
}

// updateMetrics updates producer metrics
func (p *KafkaProducer) updateMetrics(sent, errors, batches int64) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.TotalSent += sent
	p.metrics.TotalErrors += errors
	p.metrics.BatchesSent += batches
	p.metrics.LastFlushTime = time.Now()
}

// logMetrics periodically logs performance metrics
func (p *KafkaProducer) logMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.metrics.mu.RLock()
			log.Printf("Kafka Producer Metrics - Sent: %d, Errors: %d, Batches: %d, Queue Size: %d",
				p.metrics.TotalSent, p.metrics.TotalErrors, p.metrics.BatchesSent, len(p.scoreChan))
			p.metrics.mu.RUnlock()

		case <-p.ctx.Done():
			return
		}
	}
}

// ensureTopicExists checks if a topic exists and creates it if it doesn't
func (p *KafkaProducer) ensureTopicExists(topic string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to broker: %v", err)
	}
	defer conn.Close()

	topics, err := conn.ReadPartitions()
	if err != nil {
		return fmt.Errorf("failed to read topics: %v", err)
	}

	topicExists := false
	for _, t := range topics {
		if t.Topic == topic {
			topicExists = true
			break
		}
	}

	if !topicExists {
		controller, err := conn.Controller()
		if err != nil {
			return fmt.Errorf("failed to get controller: %v", err)
		}

		controllerAddr := fmt.Sprintf("%s:%d", controller.Host, controller.Port)
		controllerConn, err := kafka.DialContext(ctx, "tcp", controllerAddr)
		if err != nil {
			return fmt.Errorf("failed to connect to controller: %v", err)
		}
		defer controllerConn.Close()

		err = controllerConn.CreateTopics(kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     8, // More partitions for better parallelism
			ReplicationFactor: 1,
		})
		if err != nil {
			return fmt.Errorf("failed to create topic: %v", err)
		}
		log.Printf("Created Kafka topic: %s with 8 partitions", topic)
	} else {
		log.Printf("Kafka topic already exists: %s", topic)
	}

	return nil
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

// GetMetrics returns current producer metrics
func (p *KafkaProducer) GetMetrics() (int64, int64, int64, int) {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	return p.metrics.TotalSent, p.metrics.TotalErrors, p.metrics.BatchesSent, len(p.scoreChan)
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
