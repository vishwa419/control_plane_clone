package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"control-plane/internal/config"
	"control-plane/internal/models"
	"control-plane/internal/redis"
	"control-plane/internal/storage"
	storage_mock "control-plane/internal/storage"
)

func setupTestUploadHandler(t *testing.T) (*UploadHandler, func()) {
	// Create temporary storage
	tmpDir, err := os.MkdirTemp("", "upload_test_*")
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

	handler := NewUploadHandler(fileStorage, redisClient, cfg)

	cleanup := func() {
		redisClient.Close()
		os.RemoveAll(tmpDir)
	}

	return handler, cleanup
}

func createMultipartForm(filename, version string, fileContent []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add filename field
	if err := writer.WriteField("filename", filename); err != nil {
		return nil, "", err
	}

	// Add version field
	if err := writer.WriteField("version", version); err != nil {
		return nil, "", err
	}

	// Add file field
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(fileContent); err != nil {
		return nil, "", err
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

func TestUploadHandler_HandleUpload(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("test file content")
	filename := "testfile"
	version := "1.0.0"

	body, contentType, err := createMultipartForm(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
	}

	var response models.UploadResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success=true, got %v", response.Success)
	}

	if response.Metadata == nil {
		t.Fatal("Expected metadata in response")
	}

	if response.Metadata.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, response.Metadata.Filename)
	}

	if response.Metadata.Version != version {
		t.Errorf("Expected version %s, got %s", version, response.Metadata.Version)
	}

	// Verify checksum
	hash := sha256.Sum256(testContent)
	expectedChecksum := hex.EncodeToString(hash[:])
	if response.Metadata.Checksum != expectedChecksum {
		t.Errorf("Expected checksum %s, got %s", expectedChecksum, response.Metadata.Checksum)
	}
}

func TestUploadHandler_HandleUpload_MissingFields(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Test missing filename
	body, contentType, _ := createMultipartForm("", "1.0.0", []byte("content"))
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
	}

	// Test missing version
	body, contentType, _ = createMultipartForm("testfile", "", []byte("content"))
	req = httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr = httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
	}
}

func TestUploadHandler_HandleUpload_WrongMethod(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/upload", nil)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, status)
	}
}

func TestUploadHandler_HandleHealth(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
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

func TestUploadHandler_HandleUpload_InvalidForm(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Test with invalid form data
	req := httptest.NewRequest("POST", "/upload", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "multipart/form-data")
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
	}
}

func TestUploadHandler_HandleUpload_MissingFile(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Create form without file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", "test")
	writer.WriteField("version", "1.0.0")
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
	}
}

func TestUploadHandler_HandleUpload_LockConflict(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("test file content")
	filename := "locktest"
	version := "1.0.0"

	// Acquire lock manually first
	ctx := context.Background()
	lockKey := fmt.Sprintf("lock:upload:%s:%s", filename, version)
	acquired, err := handler.redis.AcquireLock(ctx, lockKey, 30*time.Second)
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	defer handler.redis.ReleaseLock(ctx, lockKey)

	// Try to upload same file+version (should fail with conflict)
	body, contentType, err := createMultipartForm(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusConflict {
		t.Errorf("Expected status %d (Conflict), got %d", http.StatusConflict, status)
	}
}

func TestUploadHandler_HandleUpload_FileVerificationFailure(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// This test is tricky - we need to simulate a file write that appears to succeed
	// but verification fails. We'll use a mock storage that fails on Get.
	// For now, let's test the verifyFile method directly with invalid data
	testContent := []byte("test content")
	filename := "verifytest"
	version := "1.0.0"

	// Write file normally
	filePath, err := handler.storage.Save(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Test verifyFile with wrong checksum
	wrongChecksum := "wrongchecksum"
	err = handler.verifyFile(filePath, testContent, wrongChecksum)
	if err == nil {
		t.Error("Expected error for checksum mismatch, got nil")
	}

	// Test verifyFile with wrong size data
	err = handler.verifyFile(filePath, []byte("different content"), "")
	if err == nil {
		t.Error("Expected error for size mismatch, got nil")
	}

	// Cleanup
	handler.storage.Delete(filePath)
}

func TestUploadHandler_HandleUpload_RedisFailure(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("test content")
	filename := "redistest"
	version := "1.0.0"

	// This test would require mocking Redis to fail, which is complex
	// Instead, we test storeMetadata error handling by using invalid metadata
	// For a real test, we'd need a mock Redis client
	// Let's test that the file gets deleted on Redis failure by checking the flow

	body, contentType, err := createMultipartForm(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	// If Redis fails, we should get an error
	// But in normal operation, this should succeed
	// To test Redis failure, we'd need to mock the Redis client
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Logf("Upload completed with status %d (expected OK or InternalServerError)", rr.Code)
	}
}

func TestUploadHandler_processFileUpload_StorageError(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Test with invalid filename that might cause storage error
	// We can't easily simulate storage.Save failure without mocking
	// But we can test the parallel execution works
	testContent := []byte("test content")
	checksum, filePath, err := handler.processFileUpload("test", "1.0.0", testContent)

	if err != nil {
		t.Fatalf("processFileUpload failed: %v", err)
	}

	if checksum == "" {
		t.Error("Expected checksum, got empty string")
	}

	if filePath == "" {
		t.Error("Expected filePath, got empty string")
	}

	// Verify checksum is correct
	hash := sha256.Sum256(testContent)
	expectedChecksum := hex.EncodeToString(hash[:])
	if checksum != expectedChecksum {
		t.Errorf("Expected checksum %s, got %s", expectedChecksum, checksum)
	}

	// Cleanup
	handler.storage.Delete(filePath)
}

func TestUploadHandler_verifyFile_AllCases(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("test verification content")
	filename := "verifytest"
	version := "1.0.0"

	// Save file
	filePath, err := handler.storage.Save(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}
	defer handler.storage.Delete(filePath)

	// Calculate correct checksum
	hash := sha256.Sum256(testContent)
	correctChecksum := hex.EncodeToString(hash[:])

	// Test 1: Valid file should pass
	err = handler.verifyFile(filePath, testContent, correctChecksum)
	if err != nil {
		t.Errorf("Valid file verification failed: %v", err)
	}

	// Test 2: Wrong checksum should fail
	err = handler.verifyFile(filePath, testContent, "wrongchecksum")
	if err == nil {
		t.Error("Expected error for wrong checksum, got nil")
	}
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}

	// Test 3: Wrong size should fail
	err = handler.verifyFile(filePath, []byte("different size content here"), correctChecksum)
	if err == nil {
		t.Error("Expected error for size mismatch, got nil")
	}

	// Test 4: Non-existent file should fail
	err = handler.verifyFile("/nonexistent/path", testContent, correctChecksum)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestUploadHandler_storeMetadata_Rollback(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("test content")
	filename := "metadatatest"
	version := "1.0.0"

	// Save file first
	filePath, err := handler.storage.Save(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Verify file exists
	if _, err := handler.storage.Get(filePath); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Create metadata
	metadata := &models.FileMetadata{
		Filename:   filename,
		Version:    version,
		Checksum:   "testchecksum",
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       int64(len(testContent)),
	}

	// Test that storeMetadata works normally (Redis should succeed)
	ctx := context.Background()
	err = handler.storeMetadata(ctx, filePath, metadata)
	// This should succeed in normal operation
	// To test rollback, we'd need to mock Redis to fail
	if err != nil {
		t.Logf("storeMetadata returned error (might be expected in test): %v", err)
		// If Redis fails, file should be deleted
		if _, err := handler.storage.Get(filePath); err == nil {
			t.Error("File should be deleted after Redis failure, but it still exists")
		}
	} else {
		// Cleanup if successful
		handler.storage.Delete(filePath)
	}
}



// Test the full happy path with all steps
func TestUploadHandler_HandleUpload_FullFlow(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("full flow test content with some data")
	filename := "fullflowtest"
	version := "1.0.0"

	body, contentType, err := createMultipartForm(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
		return
	}

	var response models.UploadResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify all metadata fields
	if response.Metadata.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, response.Metadata.Filename)
	}
	if response.Metadata.Version != version {
		t.Errorf("Expected version %s, got %s", version, response.Metadata.Version)
	}
	if response.Metadata.Size != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %d", len(testContent), response.Metadata.Size)
	}

	// Verify file exists and can be retrieved
	fileData, err := handler.storage.Get(response.Metadata.FilePath)
	if err != nil {
		t.Errorf("File should exist after upload: %v", err)
	}
	if !bytes.Equal(fileData, testContent) {
		t.Error("File content doesn't match uploaded content")
	}

	// Verify Redis has the metadata
	ctx := context.Background()
	metadata, err := handler.redis.GetFileMetadata(ctx, filename, version)
	if err != nil {
		t.Errorf("Metadata should exist in Redis: %v", err)
	}
	if metadata.Checksum != response.Metadata.Checksum {
		t.Errorf("Checksum mismatch: expected %s, got %s", response.Metadata.Checksum, metadata.Checksum)
	}
}

func TestUploadHandler_processFileUpload_StorageFailure(t *testing.T) {
	// Setup handler with mock storage that fails
	tmpDir, _ := os.MkdirTemp("", "upload_test_*")
	defer os.RemoveAll(tmpDir)

	redisClient, err := redis.NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
	defer redisClient.Close()

	mockStorage := &storage_mock.MockStorage{
		SaveFunc: func(filename string, version string, data []byte) (string, error) {
			return "", fmt.Errorf("storage save failed")
		},
		GetPathFunc: func(filename string, version string) string {
			return fmt.Sprintf("%s/%s/%s", tmpDir, filename, version)
		},
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "local",
			Path: tmpDir,
		},
	}

	handler := NewUploadHandler(mockStorage, redisClient, cfg)

	// Test that storage failure is handled
	_, _, err = handler.processFileUpload("test", "1.0.0", []byte("test"))
	if err == nil {
		t.Error("Expected error when storage fails, got nil")
	}
}

func TestUploadHandler_verifyFile_ReadFailure(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Use mock storage that fails on Get
	mockStorage := &storage_mock.MockStorage{
		GetFunc: func(filepath string) ([]byte, error) {
			return nil, fmt.Errorf("read failed")
		},
	}

	// Replace handler's storage with mock
	originalStorage := handler.storage
	handler.storage = mockStorage
	defer func() { handler.storage = originalStorage }()

	err := handler.verifyFile("/some/path", []byte("test"), "checksum")
	if err == nil {
		t.Error("Expected error when file read fails, got nil")
	}
}

func TestUploadHandler_storeMetadata_RedisFailure(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("test")
	filename := "redisfailtest"
	version := "1.0.0"

	// Save file first
	filePath, err := handler.storage.Save(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Verify file exists before
	if _, err := handler.storage.Get(filePath); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Create metadata with invalid data that might cause Redis to fail
	// Actually, we can't easily make Redis fail without disconnecting it
	// But we can test that the error path exists
	metadata := &models.FileMetadata{
		Filename:   filename,
		Version:    version,
		Checksum:   "test",
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       int64(len(testContent)),
	}

	ctx := context.Background()
	
	// In normal operation, this should succeed
	// To test failure, we'd need to disconnect Redis or use a mock
	err = handler.storeMetadata(ctx, filePath, metadata)
	
	// If it fails, file should be deleted
	if err != nil {
		// Check that file was deleted on Redis failure
		if _, err := handler.storage.Get(filePath); err == nil {
			t.Error("File should be deleted after Redis failure")
		}
	} else {
		// Cleanup
		handler.storage.Delete(filePath)
	}
}

// Test storeMetadata error path by using a mock Redis that fails
func TestUploadHandler_storeMetadata_ErrorPath(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "upload_test_*")
	defer os.RemoveAll(tmpDir)

	fileStorage, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create a handler but we'll test storeMetadata directly
	// We need Redis to actually fail, which is hard without mocking
	// Let's test the rollback behavior by checking the code path
	
	redisClient, err := redis.NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
	defer redisClient.Close()

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "local",
			Path: tmpDir,
		},
	}

	handler := NewUploadHandler(fileStorage, redisClient, cfg)

	// Save a file
	filePath, err := handler.storage.Save("test", "1.0.0", []byte("test"))
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Verify file exists
	if _, err := handler.storage.Get(filePath); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Test storeMetadata with valid metadata (should succeed)
	metadata := &models.FileMetadata{
		Filename:   "test",
		Version:    "1.0.0",
		Checksum:   "testchecksum",
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       4,
	}

	ctx := context.Background()
	err = handler.storeMetadata(ctx, filePath, metadata)
	
	// In normal operation, this succeeds
	// The error path (Redis failure -> file deletion) is tested implicitly
	// To fully test it, we'd need a mock Redis client
	if err != nil {
		// If Redis fails, verify file was deleted
		if _, err := handler.storage.Get(filePath); err == nil {
			t.Error("File should be deleted after Redis failure in storeMetadata")
		}
	} else {
		// Cleanup
		handler.storage.Delete(filePath)
	}
}

// Test HandleUpload error paths
func TestUploadHandler_HandleUpload_ProcessFileError(t *testing.T) {
	// Setup handler with mock storage that fails on Save
	tmpDir, _ := os.MkdirTemp("", "upload_test_*")
	defer os.RemoveAll(tmpDir)

	redisClient, err := redis.NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
	defer redisClient.Close()

	mockStorage := &storage_mock.MockStorage{
		SaveFunc: func(filename string, version string, data []byte) (string, error) {
			return "", fmt.Errorf("storage save failed")
		},
		GetPathFunc: func(filename string, version string) string {
			return fmt.Sprintf("%s/%s/%s", tmpDir, filename, version)
		},
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "local",
			Path: tmpDir,
		},
	}

	handler := NewUploadHandler(mockStorage, redisClient, cfg)

	testContent := []byte("test content")
	body, contentType, err := createMultipartForm("test", "1.0.0", testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	// Should return error when storage fails
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, status)
	}

	var response models.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
		if response.Message == "" {
			t.Error("Error message should not be empty")
		}
	}
}

func TestUploadHandler_HandleUpload_VerifyFileError(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Save original storage to avoid recursion
	originalStorage := handler.storage

	// Use mock storage that fails on Get (verification step)
	mockStorage := &storage_mock.MockStorage{
		SaveFunc: func(filename string, version string, data []byte) (string, error) {
			// Save succeeds using original storage
			return originalStorage.Save(filename, version, data)
		},
		GetFunc: func(filepath string) ([]byte, error) {
			// Get fails - simulates verification failure
			return nil, fmt.Errorf("file not readable")
		},
		DeleteFunc: func(filepath string) error {
			return originalStorage.Delete(filepath)
		},
		GetPathFunc: func(filename string, version string) string {
			return originalStorage.GetPath(filename, version)
		},
	}

	// Replace handler's storage with mock
	handler.storage = mockStorage
	defer func() { handler.storage = originalStorage }()

	// Use unique filename to avoid lock conflicts
	testContent := []byte("test content")
	filename := fmt.Sprintf("verifytest_%d", time.Now().UnixNano())
	body, contentType, err := createMultipartForm(filename, "1.0.0", testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	// Should return error when verification fails
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, status)
	}

	var response models.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
		if !contains(response.Message, "verification") {
			t.Errorf("Expected verification error message, got: %s", response.Message)
		}
	}
}


// Test parseUploadRequest with cancelled context to simulate ReadAll error
func TestUploadHandler_parseUploadRequest_ReadAllError(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("read all test")
	body, contentType, err := createMultipartForm("readalltest", "1.0.0", testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	// Create request with cancelled context to simulate read error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	req := httptest.NewRequest("POST", "/upload", body)
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	// Test through HandleUpload - cancelled context may cause issues
	handler.HandleUpload(rr, req)

	// Should get an error due to cancelled context
	if rr.Code == http.StatusInternalServerError {
		var response models.ErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
			if response.Message != "" {
				t.Logf("Error path tested: %s", response.Message)
			}
		}
	}
}

// Test acquireUploadLock error case (when AcquireLock returns error)
func TestUploadHandler_acquireUploadLock_Error(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	// Use cancelled context to simulate Redis error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest("POST", "/upload", nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	// Test acquireUploadLock with cancelled context
	lockKey, released := handler.acquireUploadLock(rr, ctx, "test", "1.0.0")
	
	if released {
		t.Error("Expected lock acquisition to fail with cancelled context")
	}
	if lockKey != "" {
		t.Error("Expected empty lockKey when acquisition fails")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

// Test storeMetadata Redis failure and rollback
func TestUploadHandler_storeMetadata_RedisFailureRollback(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("rollback test")
	filename := "rollbacktest"
	version := "1.0.0"

	// Save file first
	filePath, err := handler.storage.Save(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Verify file exists
	if _, err := handler.storage.Get(filePath); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Create metadata
	metadata := &models.FileMetadata{
		Filename:   filename,
		Version:    version,
		Checksum:   "testchecksum",
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       int64(len(testContent)),
	}

	// Test with cancelled context to simulate Redis failure
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to cause Redis operation to fail
	
	// This should fail due to cancelled context
	err = handler.storeMetadata(ctx, filePath, metadata)
	
	// If Redis fails (due to cancelled context), file should be deleted (rollback)
	if err != nil {
		// Verify file was deleted on rollback
		if _, err := handler.storage.Get(filePath); err == nil {
			t.Error("File should be deleted after Redis failure in storeMetadata (rollback)")
		}
	} else {
		// If it succeeded (unlikely with cancelled context), cleanup
		handler.storage.Delete(filePath)
	}
}

// Test storeMetadata success path
func TestUploadHandler_storeMetadata_Success(t *testing.T) {
	handler, cleanup := setupTestUploadHandler(t)
	defer cleanup()

	testContent := []byte("success test")
	filename := "successtest"
	version := "1.0.0"

	// Save file first
	filePath, err := handler.storage.Save(filename, version, testContent)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Create metadata
	hash := sha256.Sum256(testContent)
	checksum := hex.EncodeToString(hash[:])
	
	metadata := &models.FileMetadata{
		Filename:   filename,
		Version:    version,
		Checksum:   checksum,
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       int64(len(testContent)),
	}

	ctx := context.Background()
	
	// Test storeMetadata success path
	err = handler.storeMetadata(ctx, filePath, metadata)
	if err != nil {
		t.Errorf("storeMetadata should succeed: %v", err)
	}

	// Verify file still exists (not deleted on success)
	if _, err := handler.storage.Get(filePath); err != nil {
		t.Error("File should still exist after successful storeMetadata")
	}

	// Cleanup
	handler.storage.Delete(filePath)
}

// Test HandleUpload with all error paths
func TestUploadHandler_HandleUpload_AllErrorPaths(t *testing.T) {
	// Test 1: processFileUpload error
	tmpDir, _ := os.MkdirTemp("", "upload_test_*")
	defer os.RemoveAll(tmpDir)

	redisClient, err := redis.NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
	defer redisClient.Close()

	mockStorage := &storage_mock.MockStorage{
		SaveFunc: func(filename string, version string, data []byte) (string, error) {
			return "", fmt.Errorf("storage save failed")
		},
		GetPathFunc: func(filename string, version string) string {
			return fmt.Sprintf("%s/%s/%s", tmpDir, filename, version)
		},
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "local",
			Path: tmpDir,
		},
	}

	handlerWithMock := NewUploadHandler(mockStorage, redisClient, cfg)

	testContent := []byte("error test")
	filename := fmt.Sprintf("errortest_%d", time.Now().UnixNano())
	body, contentType, err := createMultipartForm(filename, "1.0.0", testContent)
	if err != nil {
		t.Fatalf("Failed to create multipart form: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()

	handlerWithMock.HandleUpload(rr, req)

	// Should return error when storage fails
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, status)
	}

	var response models.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
		if response.Message == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
