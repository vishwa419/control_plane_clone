package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"control-plane/internal/config"
	"control-plane/internal/models"
	"control-plane/internal/redis"
	"control-plane/internal/storage"
)

func setupTestConsumerHandler(t *testing.T) (*ConsumerHandler, func()) {
	// Create temporary storage
	tmpDir, err := os.MkdirTemp("", "consumer_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	fileStorage, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Try to connect to Redis (skip if not available)
	redisClient, err := redis.NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "local",
			Path: tmpDir,
		},
	}

	handler := NewConsumerHandler(fileStorage, redisClient, cfg)

	cleanup := func() {
		redisClient.Close()
		os.RemoveAll(tmpDir)
	}

	return handler, cleanup
}

func setupTestFile(t *testing.T, handler *ConsumerHandler, filename, version string, content []byte) {
	// Store file in storage
	filePath, err := handler.storage.Save(filename, version, content)
	if err != nil {
		t.Fatalf("Failed to save test file: %v", err)
	}

	// Store metadata in Redis
	metadata := &models.FileMetadata{
		Filename:   filename,
		Version:    version,
		Checksum:   "test-checksum",
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       int64(len(content)),
	}

	ctx := context.Background()
	if err := handler.redis.StoreFileMetadata(ctx, metadata); err != nil {
		t.Fatalf("Failed to store metadata: %v", err)
	}
}

func TestConsumerHandler_HandleGetFile(t *testing.T) {
	handler, cleanup := setupTestConsumerHandler(t)
	defer cleanup()

	testContent := []byte("test file content")
	filename := "testfile"
	version := "1.0.0"

	setupTestFile(t, handler, filename, version, testContent)

	req := httptest.NewRequest("GET", "/file/"+filename, nil)
	req = mux.SetURLVars(req, map[string]string{"filename": filename})
	rr := httptest.NewRecorder()

	handler.HandleGetFile(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
	}

	if !bytes.Equal(rr.Body.Bytes(), testContent) {
		t.Errorf("Response body doesn't match. Expected %s, got %s", string(testContent), string(rr.Body.Bytes()))
	}

	// Check headers
	if rr.Header().Get("X-File-Version") != version {
		t.Errorf("Expected X-File-Version header %s, got %s", version, rr.Header().Get("X-File-Version"))
	}
}

func TestConsumerHandler_HandleGetFile_Nonexistent(t *testing.T) {
	handler, cleanup := setupTestConsumerHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/file/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"filename": "nonexistent"})
	rr := httptest.NewRecorder()

	handler.HandleGetFile(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, status)
	}
}

func TestConsumerHandler_HandleGetFileVersion(t *testing.T) {
	handler, cleanup := setupTestConsumerHandler(t)
	defer cleanup()

	testContent := []byte("version 1.0.0 content")
	filename := "testfile"
	version := "1.0.0"

	setupTestFile(t, handler, filename, version, testContent)

	req := httptest.NewRequest("GET", "/file/"+filename+"/version/"+version, nil)
	req = mux.SetURLVars(req, map[string]string{
		"filename": filename,
		"version":  version,
	})
	rr := httptest.NewRecorder()

	handler.HandleGetFileVersion(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !bytes.Equal(rr.Body.Bytes(), testContent) {
		t.Errorf("Response body doesn't match")
	}
}

func TestConsumerHandler_HandleGetFileInfo(t *testing.T) {
	handler, cleanup := setupTestConsumerHandler(t)
	defer cleanup()

	testContent := []byte("test content")
	filename := "testfile"
	version := "1.0.0"

	setupTestFile(t, handler, filename, version, testContent)

	req := httptest.NewRequest("GET", "/file/"+filename+"/info", nil)
	req = mux.SetURLVars(req, map[string]string{"filename": filename})
	rr := httptest.NewRecorder()

	handler.HandleGetFileInfo(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
	}

	var response models.FileInfoResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, response.Filename)
	}

	if response.Version != version {
		t.Errorf("Expected version %s, got %s", version, response.Version)
	}
}

func TestConsumerHandler_HandleHealth(t *testing.T) {
	handler, cleanup := setupTestConsumerHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.HandleHealth(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", response["status"])
	}
}
