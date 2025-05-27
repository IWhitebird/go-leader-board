package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IWhitebird/go-leader-board/config"
	"github.com/IWhitebird/go-leader-board/internal/logging"
	"github.com/IWhitebird/go-leader-board/internal/models"
	"github.com/IWhitebird/go-leader-board/internal/store"
	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader        *kafka.Reader
	store         *store.Store
	batchSize     int
	timeout       time.Duration
	brokers       []string
	topic         string
	consumerGroup string
}

func NewKafkaConsumer(cfg *config.AppConfig, store *store.Store) (*KafkaConsumer, error) {
	consumer := &KafkaConsumer{
		store:         store,
		batchSize:     cfg.Kafka.BatchSize,
		timeout:       time.Duration(cfg.Kafka.BatchTimeout) * time.Second,
		brokers:       cfg.Kafka.Brokers,
		topic:         cfg.Kafka.ScoresTopicPrefix,
		consumerGroup: fmt.Sprintf("%s-%s", cfg.Kafka.ConsumerGroup, cfg.Kafka.ServiceID),
	}

	// Retry connecting to Kafka
	maxRetries := 5
	var err error
	for i := range maxRetries {
		if err = consumer.connect(); err == nil {
			break
		}
		logging.Error("Failed to connect consumer to Kafka", "attempt", i+1, "max", maxRetries, "error", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect consumer to Kafka after %d attempts: %v", maxRetries, err)
	}

	return consumer, nil
}

func (c *KafkaConsumer) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", c.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

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
		logging.Error("Topic does not exist, consumer may not function correctly", "topic", c.topic)
	}
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
	logging.Info("Created Kafka consumer", "topic", c.topic, "group", c.consumerGroup)
	return nil
}

func (c *KafkaConsumer) StartConsumer(ctx context.Context) {
	logging.Info("Starting Kafka consumer", "topic", c.topic)

	go func() {
		defer c.reader.Close()

		for {
			select {
			case <-ctx.Done():
				logging.Info("Kafka consumer shutting down")
				return
			default:
				if err := c.processBatch(ctx); err != nil {
					logging.Error("Error processing batch", "error", err)
					time.Sleep(time.Second * 2)
				}
			}
		}
	}()
}

func (c *KafkaConsumer) processBatch(ctx context.Context) error {
	batch := make([]models.Score, 0, c.batchSize)
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()

	batchCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	for len(batch) < c.batchSize {
		select {
		case <-timer.C:
			if len(batch) > 0 {
				return c.saveBatch(batch)
			}
			return nil
		case <-ctx.Done():
			if len(batch) > 0 {
				return c.saveBatch(batch)
			}
			return ctx.Err()
		default:
			fetchCtx, fetchCancel := context.WithTimeout(batchCtx, 100*time.Millisecond)
			message, err := c.reader.FetchMessage(fetchCtx)
			fetchCancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				return fmt.Errorf("error fetching message from Kafka: %v", err)
			}

			var score models.Score
			if err := json.Unmarshal(message.Value, &score); err != nil {
				logging.Error("Error unmarshaling score", "error", err)
				if commitErr := c.reader.CommitMessages(ctx, message); commitErr != nil {
					logging.Error("Error committing invalid message", "error", commitErr)
				}
				continue
			}

			batch = append(batch, score)

			if err := c.reader.CommitMessages(ctx, message); err != nil {
				return fmt.Errorf("error committing message: %v", err)
			}
		}
	}

	if len(batch) > 0 {
		return c.saveBatch(batch)
	}

	return nil
}

func (c *KafkaConsumer) saveBatch(batch []models.Score) error {
	logging.Info("Saving batch of scores", "count", len(batch))

	if len(batch) == 0 {
		return nil
	}

	if err := c.store.SaveScoreBatch(batch); err != nil {
		logging.Error("Error saving batch", "error", err)
		return fmt.Errorf("failed to save batch: %v", err)
	}

	return nil
}

func (c *KafkaConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
