package grpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"control-plane/proto/gen"
	"google.golang.org/grpc"
)

// Server wraps the gRPC server
type Server struct {
	grpcServer    *grpc.Server
	streamManager *StreamManager
	port          string
}

// NewServer creates and configures a new gRPC server
func NewServer(port string) *Server {
	streamManager := NewStreamManager()

	grpcServer := grpc.NewServer()
	controlPlaneService := NewControlPlaneService(streamManager)
	gen.RegisterControlPlaneServer(grpcServer, controlPlaneService)

	return &Server{
		grpcServer:    grpcServer,
		streamManager: streamManager,
		port:          port,
	}
}

// Start starts the gRPC server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	log.Printf("gRPC server starting on %s", addr)
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	log.Println("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
	log.Println("gRPC server stopped")
}

// GetStreamManager returns the stream manager (for integration with upload handler)
func (s *Server) GetStreamManager() *StreamManager {
	return s.streamManager
}

// BroadcastWorkerUpdate broadcasts a worker update to all consumers
// This can be called from upload handler via HTTP or direct function call
func (s *Server) BroadcastWorkerUpdate(workerName, version, checksum, filePath string, workerCode []byte, size int64, timestamp int64) error {
	update := &gen.WorkerUpdate{
		WorkerName: workerName,
		Version:    version,
		Checksum:   checksum,
		WorkerCode: workerCode,
		Size:       size,
		FilePath:   filePath,
		Timestamp:  timestamp,
	}

	return s.streamManager.Broadcast(update)
}

// SetupHTTPBroadcastEndpoint sets up an HTTP endpoint for broadcast (for cross-container communication)
func (s *Server) SetupHTTPBroadcastEndpoint(mux *http.ServeMux) {
	mux.HandleFunc("/internal/broadcast", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		// Get metadata JSON
		metadataJSON := r.FormValue("metadata")
		if metadataJSON == "" {
			http.Error(w, "metadata is required", http.StatusBadRequest)
			return
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			http.Error(w, fmt.Sprintf("Invalid metadata JSON: %v", err), http.StatusBadRequest)
			return
		}

		// Get worker code from form file
		file, _, err := r.FormFile("worker_code")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get worker_code: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()

		workerCode, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read worker_code: %v", err), http.StatusInternalServerError)
			return
		}

		// Extract fields from metadata
		workerName, _ := metadata["worker_name"].(string)
		version, _ := metadata["version"].(string)
		checksum, _ := metadata["checksum"].(string)
		filePath, _ := metadata["file_path"].(string)
		size, _ := metadata["size"].(float64)
		timestamp, _ := metadata["timestamp"].(float64)

		// Broadcast update
		update := &gen.WorkerUpdate{
			WorkerName: workerName,
			Version:    version,
			Checksum:   checksum,
			WorkerCode: workerCode,
			Size:       int64(size),
			FilePath:   filePath,
			Timestamp:  int64(timestamp),
		}

		if err := s.streamManager.Broadcast(update); err != nil {
			http.Error(w, fmt.Sprintf("Failed to broadcast: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Broadcast successful for %s v%s", workerName, version)
	})
}
