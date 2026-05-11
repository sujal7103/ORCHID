package services

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/redis/go-redis/v9"
)

// PubSubService manages Redis pub/sub for cross-instance communication
type PubSubService struct {
	redis        *RedisService
	pubsub       *redis.PubSub
	handlers     map[string][]MessageHandler
	mu           sync.RWMutex
	instanceID   string
	ctx          context.Context
	cancel       context.CancelFunc
}

// MessageHandler is a callback for handling pub/sub messages
type MessageHandler func(channel string, message *PubSubMessage)

// PubSubMessage represents a message sent via pub/sub
type PubSubMessage struct {
	Type       string                 `json:"type"`       // Message type (e.g., "execution_update", "agent_status")
	UserID     string                 `json:"userId"`     // Target user ID
	AgentID    string                 `json:"agentId,omitempty"`
	InstanceID string                 `json:"instanceId"` // Source instance ID
	Payload    map[string]interface{} `json:"payload"`    // Message payload
}

// NewPubSubService creates a new pub/sub service
func NewPubSubService(redisService *RedisService, instanceID string) *PubSubService {
	ctx, cancel := context.WithCancel(context.Background())
	return &PubSubService{
		redis:      redisService,
		handlers:   make(map[string][]MessageHandler),
		instanceID: instanceID,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Subscribe subscribes to a channel pattern
func (s *PubSubService) Subscribe(pattern string, handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers[pattern] = append(s.handlers[pattern], handler)
	log.Printf("📡 [PUBSUB] Subscribed to pattern: %s", pattern)
}

// Start begins listening for pub/sub messages
func (s *PubSubService) Start() error {
	client := s.redis.Client()

	// Subscribe to all user channels and global channels
	s.pubsub = client.PSubscribe(s.ctx,
		"user:*:events",      // User-specific events
		"agent:*:events",     // Agent-specific events
		"broadcast:*",        // Global broadcast
	)

	// Wait for subscription confirmation
	_, err := s.pubsub.Receive(s.ctx)
	if err != nil {
		return err
	}

	// Start message processor
	go s.processMessages()

	log.Printf("✅ [PUBSUB] Started listening for messages (instance: %s)", s.instanceID)
	return nil
}

// processMessages handles incoming pub/sub messages
func (s *PubSubService) processMessages() {
	ch := s.pubsub.Channel()

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.handleMessage(msg)
		}
	}
}

// handleMessage processes a single pub/sub message
func (s *PubSubService) handleMessage(msg *redis.Message) {
	var message PubSubMessage
	if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
		log.Printf("⚠️ [PUBSUB] Failed to unmarshal message: %v", err)
		return
	}

	// Skip messages from this instance (avoid loops)
	if message.InstanceID == s.instanceID {
		return
	}

	// Find matching handlers
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check for exact match
	if handlers, ok := s.handlers[msg.Channel]; ok {
		for _, handler := range handlers {
			go handler(msg.Channel, &message)
		}
	}

	// Check for pattern matches (simplified - real implementation would use glob matching)
	for pattern, handlers := range s.handlers {
		if matchPattern(pattern, msg.Channel) {
			for _, handler := range handlers {
				go handler(msg.Channel, &message)
			}
		}
	}
}

// PublishToUser publishes a message to a user's channel
func (s *PubSubService) PublishToUser(ctx context.Context, userID string, msgType string, payload map[string]interface{}) error {
	message := &PubSubMessage{
		Type:       msgType,
		UserID:     userID,
		InstanceID: s.instanceID,
		Payload:    payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	channel := "user:" + userID + ":events"
	return s.redis.Client().Publish(ctx, channel, data).Err()
}

// PublishToAgent publishes a message to an agent's channel
func (s *PubSubService) PublishToAgent(ctx context.Context, agentID string, msgType string, payload map[string]interface{}) error {
	message := &PubSubMessage{
		Type:       msgType,
		AgentID:    agentID,
		InstanceID: s.instanceID,
		Payload:    payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	channel := "agent:" + agentID + ":events"
	return s.redis.Client().Publish(ctx, channel, data).Err()
}

// Broadcast publishes a message to all instances
func (s *PubSubService) Broadcast(ctx context.Context, topic string, msgType string, payload map[string]interface{}) error {
	message := &PubSubMessage{
		Type:       msgType,
		InstanceID: s.instanceID,
		Payload:    payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	channel := "broadcast:" + topic
	return s.redis.Client().Publish(ctx, channel, data).Err()
}

// PublishExecutionUpdate publishes an execution update for a user
func (s *PubSubService) PublishExecutionUpdate(ctx context.Context, userID, agentID, executionID string, update map[string]interface{}) error {
	payload := map[string]interface{}{
		"executionId": executionID,
		"agentId":     agentID,
		"update":      update,
	}

	return s.PublishToUser(ctx, userID, "execution_update", payload)
}

// Stop stops the pub/sub service
func (s *PubSubService) Stop() error {
	s.cancel()
	if s.pubsub != nil {
		return s.pubsub.Close()
	}
	return nil
}

// matchPattern checks if a channel matches a pattern (simplified glob)
func matchPattern(pattern, channel string) bool {
	// Simple wildcard matching
	if pattern == channel {
		return true
	}

	// Handle patterns like "user:*:events"
	patternParts := splitChannel(pattern)
	channelParts := splitChannel(channel)

	if len(patternParts) != len(channelParts) {
		return false
	}

	for i, part := range patternParts {
		if part != "*" && part != channelParts[i] {
			return false
		}
	}

	return true
}

// splitChannel splits a channel name by ":"
func splitChannel(channel string) []string {
	var parts []string
	current := ""
	for _, c := range channel {
		if c == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}
