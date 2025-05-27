package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IWhitebird/go-leader-board/config"
	"github.com/IWhitebird/go-leader-board/internal/logging"
	"github.com/IWhitebird/go-leader-board/internal/models"
	"github.com/segmentio/kafka-go"
)

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

func NewKafkaProducer(cfg *config.AppConfig) (*KafkaProducer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        cfg.Kafka.ScoresTopicPrefix,
		Balancer:     &kafka.Hash{},
		BatchSize:    5000,
		BatchBytes:   1024 * 1024 * 2,
		BatchTimeout: 500 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Async:        true,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,
		Compression:  kafka.Snappy,
		MaxAttempts:  3,
	}

	producer := &KafkaProducer{
		writer:        writer,
		connected:     false,
		scoreChan:     make(chan models.Score, 20000),
		ctx:           ctx,
		cancel:        cancel,
		batchSize:     5000,
		flushInterval: 1 * time.Second,
	}

	maxRetries := 5
	var err error
	for i := range maxRetries {
		if err = producer.testConnection(cfg.Kafka.Brokers); err == nil {
			break
		}
		logging.Error("Failed to connect to Kafka", "attempt", i+1, "max", maxRetries, "error", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Kafka after %d attempts: %v", maxRetries, err)
	}

	producer.connected = true
	producer.startBatchProcessor()
	return producer, nil
}

func (p *KafkaProducer) testConnection(brokers []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	logging.Info("Successfully connected to Kafka cluster")
	return nil
}

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

				if len(batch) >= p.batchSize {
					p.flushBatch(batch)
					batch = batch[:0]
				}

			case <-ticker.C:
				if len(batch) > 0 {
					p.flushBatch(batch)
					batch = batch[:0]
				}

			case <-p.ctx.Done():
				if len(batch) > 0 {
					p.flushBatch(batch)
				}
				return
			}
		}
	}()
}

func (p *KafkaProducer) flushBatch(scores []models.Score) {
	if len(scores) == 0 {
		return
	}

	messages := make([]kafka.Message, len(scores))
	for i, score := range scores {
		scoreJSON, err := json.Marshal(score)
		if err != nil {
			logging.Error("Error marshaling score", "error", err)
			continue
		}

		messages[i] = kafka.Message{
			Key:   []byte(fmt.Sprintf("game-%d", score.GameID)),
			Value: scoreJSON,
			Time:  time.Now(),
		}
	}

	ctx, cancel := context.WithTimeout(p.ctx, 15*time.Second)
	defer cancel()

	start := time.Now()
	err := p.writer.WriteMessages(ctx, messages...)
	duration := time.Since(start)

	if err != nil {
		logging.Error("Error sending batch to Kafka", "count", len(messages), "duration", duration, "error", err)
	} else {
		logging.Info("Successfully sent batch to Kafka", "count", len(messages), "duration", duration)
	}
}

func (p *KafkaProducer) SendScore(ctx context.Context, score models.Score) error {
	p.mu.RLock()
	connected := p.connected
	p.mu.RUnlock()

	if !connected {
		return fmt.Errorf("producer not connected")
	}

	select {
	case p.scoreChan <- score:
		return nil
	default:
		return fmt.Errorf("producer queue full - too many concurrent writes")
	}
}

func (p *KafkaProducer) Close() error {
	logging.Info("Shutting down Kafka producer")

	p.mu.Lock()
	p.connected = false
	p.mu.Unlock()

	p.cancel()
	p.wg.Wait()

	if p.writer != nil {
		err := p.writer.Close()
		logging.Info("Kafka producer shutdown complete")
		return err
	}

	return nil
}
