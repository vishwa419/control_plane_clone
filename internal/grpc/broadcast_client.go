package grpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// BroadcastClient is a client for broadcasting worker updates to the gRPC server
type BroadcastClient struct {
	grpcServerURL string
	httpClient    *http.Client
}

// NewBroadcastClient creates a new broadcast client
func NewBroadcastClient(grpcServerURL string) *BroadcastClient {
	return &BroadcastClient{
		grpcServerURL: grpcServerURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// BroadcastUpdate sends a worker update to the gRPC server for broadcasting
func (c *BroadcastClient) BroadcastUpdate(workerName, version, checksum, filePath string, workerCode []byte, size int64) error {
	// Create request payload
	payload := map[string]interface{}{
		"worker_name": workerName,
		"version":     version,
		"checksum":    checksum,
		"file_path":   filePath,
		"size":        size,
		"timestamp":   time.Now().Unix(),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create multipart form for file data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add JSON metadata
	if err := writer.WriteField("metadata", string(jsonData)); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Add worker code as file
	part, err := writer.CreateFormFile("worker_code", workerName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(workerCode); err != nil {
		return fmt.Errorf("failed to write worker code: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Make HTTP request
	url := fmt.Sprintf("%s/internal/broadcast", c.grpcServerURL)
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("broadcast failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
