package dispatcher

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/alertowls/backend-go/internal/database"
	"github.com/alertowls/backend-go/internal/models"
	"github.com/alertowls/backend-go/internal/sse"
	"github.com/redis/go-redis/v9"
)

type RedisAlertConsumer struct {
	client     *redis.Client
	db         *database.DB
	hub        *sse.Hub
	dispatcher *Dispatcher
	stopChan   chan struct{}
}

func StartRedisConsumer(redisURL string, db *database.DB, hub *sse.Hub, disp *Dispatcher) *RedisAlertConsumer {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[Redis Consumer] Invalid Redis URL (%s): %v. Running in HTTP-only mode.", redisURL, err)
		return nil
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[Redis Consumer] Redis not reachable (%v). Running in HTTP fallback mode.", err)
		_ = client.Close()
		return nil
	}

	log.Printf("[Redis Consumer] Connected to Redis at %s. Listening on 'alerts_stream'...", redisURL)
	consumer := &RedisAlertConsumer{
		client:     client,
		db:         db,
		hub:        hub,
		dispatcher: disp,
		stopChan:   make(chan struct{}),
	}

	go consumer.loop()
	return consumer
}

func (c *RedisAlertConsumer) loop() {
	streamName := "alerts_stream"
	lastID := "$"

	for {
		select {
		case <-c.stopChan:
			return
		default:
			ctx := context.Background()
			res, err := c.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{streamName, lastID},
				Count:   10,
				Block:   2 * time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil {
					time.Sleep(1 * time.Second)
				}
				continue
			}

			for _, stream := range res {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					payloadStr, ok := msg.Values["payload"].(string)
					if !ok {
						continue
					}

					var aiLead models.AIProcessedLead
					if err := json.Unmarshal([]byte(payloadStr), &aiLead); err != nil {
						log.Printf("[Redis Consumer] Error decoding payload: %v", err)
						continue
					}

					if aiLead.IsNoise || aiLead.Intent == models.IntentNoise {
						continue
					}

					exists, _ := c.db.LeadExists(aiLead.ExternalID)
					if exists {
						continue
					}

					if aiLead.UserID == "" {
						u, _ := c.db.GetDefaultUser()
						if u != nil {
							aiLead.UserID = u.ID
						}
					}

					saved, err := c.db.SaveLead(&aiLead)
					if err != nil {
						log.Printf("[Redis Consumer] Error saving lead: %v", err)
						continue
					}

					c.hub.BroadcastLead(saved)

					user, _ := c.db.GetUser(saved.UserID)
					if user != nil {
						c.dispatcher.DispatchLead(user, saved)
					}
				}
			}
		}
	}
}

func (c *RedisAlertConsumer) Stop() {
	if c != nil && c.client != nil {
		close(c.stopChan)
		_ = c.client.Close()
	}
}
