package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"control-plane/proto/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ControlPlaneService implements the ControlPlane gRPC service
type ControlPlaneService struct {
	gen.UnimplementedControlPlaneServer
	streamManager *StreamManager
}

// NewControlPlaneService creates a new ControlPlane service implementation
func NewControlPlaneService(streamManager *StreamManager) *ControlPlaneService {
	return &ControlPlaneService{
		streamManager: streamManager,
	}
}

// SubscribeWorkerUpdates streams worker updates to a consumer
func (s *ControlPlaneService) SubscribeWorkerUpdates(req *gen.SubscribeRequest, stream gen.ControlPlane_SubscribeWorkerUpdatesServer) error {
	consumerID := req.ConsumerId
	if consumerID == "" {
		return status.Error(codes.InvalidArgument, "consumer_id is required")
	}

	log.Printf("Consumer %s subscribing to worker updates", consumerID)

	// Register consumer and get update channel
	updateChan := s.streamManager.Register(consumerID)
	defer s.streamManager.Unregister(consumerID)

	// Update last seen periodically (every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Update last seen in Redis if redis client is available
				// This would require passing redis client to the service
			case <-stream.Context().Done():
				return
			}
		}
	}()

	// Send updates as they arrive
	for update := range updateChan {
		// Check if consumer wants specific workers (filtering)
		if len(req.WorkerNames) > 0 {
			// Check if this update is for a requested worker
			found := false
			for _, workerName := range req.WorkerNames {
				if workerName == update.WorkerName {
					found = true
					break
				}
			}
			if !found {
				continue // Skip this update
			}
		}

		// Send update to consumer
		if err := stream.Send(update); err != nil {
			log.Printf("Error sending update to consumer %s: %v", consumerID, err)
			return err
		}
		log.Printf("Sent update for %s v%s to consumer %s", update.WorkerName, update.Version, consumerID)
	}

	log.Printf("Consumer %s disconnected", consumerID)
	return nil
}

// RegisterConsumer registers a consumer with the control plane
func (s *ControlPlaneService) RegisterConsumer(ctx context.Context, req *gen.RegisterRequest) (*gen.RegisterResponse, error) {
	consumerID := req.ConsumerId
	if consumerID == "" {
		return nil, status.Error(codes.InvalidArgument, "consumer_id is required")
	}

	// Consumer registration is implicit when they call SubscribeWorkerUpdates
	// This endpoint can be used for health checks or metadata
	log.Printf("Consumer %s registered at endpoint %s", consumerID, req.Endpoint)

	// Note: Redis registration would happen here if redis client was available
	// For now, registration happens implicitly via SubscribeWorkerUpdates

	return &gen.RegisterResponse{
		Success: true,
		Message: fmt.Sprintf("Consumer %s registered successfully", consumerID),
	}, nil
}
