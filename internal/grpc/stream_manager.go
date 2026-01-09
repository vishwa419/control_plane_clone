package grpc

import (
	"fmt"
	"sync"

	"control-plane/proto/gen"
)

// StreamManager manages consumer connections and broadcasts worker updates
type StreamManager struct {
	streams map[string]chan *gen.WorkerUpdate
	mu      sync.RWMutex
}

// NewStreamManager creates a new StreamManager
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[string]chan *gen.WorkerUpdate),
	}
}

// Register registers a consumer and returns a channel for receiving updates
func (sm *StreamManager) Register(consumerID string) chan *gen.WorkerUpdate {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Create buffered channel for updates
	updateChan := make(chan *gen.WorkerUpdate, 100) // Buffer 100 updates
	sm.streams[consumerID] = updateChan

	return updateChan
}

// Unregister removes a consumer from the registry
func (sm *StreamManager) Unregister(consumerID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if ch, exists := sm.streams[consumerID]; exists {
		close(ch) // Close channel to signal consumer goroutine to exit
		delete(sm.streams, consumerID)
	}
}

// Broadcast sends a worker update to all registered consumers
func (sm *StreamManager) Broadcast(update *gen.WorkerUpdate) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.streams) == 0 {
		return nil // No consumers connected
	}

	var errors []error
	for consumerID, ch := range sm.streams {
		select {
		case ch <- update:
			// Successfully sent
		default:
			// Channel buffer full - consumer might be slow
			// Log warning but don't block
			errors = append(errors, fmt.Errorf("consumer %s channel full, update dropped", consumerID))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send to %d consumers: %v", len(errors), errors)
	}

	return nil
}

// GetActiveConsumerCount returns the number of active consumers
func (sm *StreamManager) GetActiveConsumerCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.streams)
}

// GetConsumerIDs returns all registered consumer IDs
func (sm *StreamManager) GetConsumerIDs() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := make([]string, 0, len(sm.streams))
	for id := range sm.streams {
		ids = append(ids, id)
	}
	return ids
}
