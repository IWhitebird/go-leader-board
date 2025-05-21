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

// KafkaProducer handles score message production to Kafka
type KafkaProducer struct {
	writer    *kafka.Writer
	topic     string
	mu        sync.Mutex
	connected bool
	brokers   []string
}

// NewKafkaProducer creates a new Kafka producer
func NewKafkaProducer(cfg *config.AppConfig) (*KafkaProducer, error) {
	producer := &KafkaProducer{
		topic:     cfg.Kafka.ScoresTopic,
		connected: false,
		brokers:   cfg.Kafka.Brokers,
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
		return nil, fmt.Errorf("failed to connect to Kafka after %d attempts: %v", maxRetries, err)
	}

	// Ensure topic exists
	if err := producer.ensureTopicExists(cfg.Kafka.ScoresTopic); err != nil {
		log.Printf("Warning: could not verify topic exists: %v", err)
	}

	return producer, nil
}

// connect establishes connection to Kafka
func (p *KafkaProducer) connect() error {
	// Create Kafka writer
	writer := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        p.topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1, // For scores, immediate delivery is usually better
		RequiredAcks: kafka.RequireOne,
		// Add retry config
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	p.writer = writer
	p.connected = true

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping Kafka by listing topics
	conn, err := kafka.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	return nil
}

// ensureTopicExists checks if a topic exists and creates it if it doesn't
func (p *KafkaProducer) ensureTopicExists(topic string) error {
	// Connect to one of the brokers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to broker: %v", err)
	}
	defer conn.Close()

	// List topics to check if our topic exists
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

	// Create topic if it doesn't exist
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
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
		if err != nil {
			return fmt.Errorf("failed to create topic: %v", err)
		}
		log.Printf("Created Kafka topic: %s", topic)
	} else {
		log.Printf("Kafka topic already exists: %s", topic)
	}

	return nil
}

// SendScore sends a score to Kafka
func (p *KafkaProducer) SendScore(ctx context.Context, score models.Score) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.connected {
		return fmt.Errorf("producer not connected")
	}

	// Convert score to JSON
	scoreJSON, err := json.Marshal(score)
	if err != nil {
		return err
	}

	// Send message
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%d-%d", score.GameID, score.UserID)),
		Value: scoreJSON,
		Time:  time.Now(),
	}

	return p.writer.WriteMessages(ctx, msg)
}

// Close closes the Kafka producer
func (p *KafkaProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connected {
		p.connected = false
		return p.writer.Close()
	}
	return nil
}
