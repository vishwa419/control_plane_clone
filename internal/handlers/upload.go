package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"control-plane/internal/config"
	"control-plane/internal/models"
	"control-plane/internal/redis"
	"control-plane/internal/storage"
)

// UploadHandler handles file upload requests
type UploadHandler struct {
	storage storage.Storage
	redis   *redis.Client
	config  *config.Config
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(storage storage.Storage, redisClient *redis.Client, cfg *config.Config) *UploadHandler {
	return &UploadHandler{
		storage: storage,
		redis:   redisClient,
		config:  cfg,
	}
}

// HandleUpload handles POST /upload requests
func (h *UploadHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if !h.validateRequest(w, r) {
		return
	}

	// Parse upload request
	filename, version, fileData, header, err := h.parseUploadRequest(w, r)
	if err != nil {
		return // Error already responded
	}

	ctx := r.Context()

	// Acquire lock and ensure release
	lockKey, released := h.acquireUploadLock(w, ctx, filename, version)
	if !released {
		return // Error already responded or lock not acquired
	}
	defer h.redis.ReleaseLock(ctx, lockKey)

	// Process file upload (checksum + storage in parallel)
	checksum, filePath, err := h.processFileUpload(filename, version, fileData)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to process file", err)
		return
	}

	// Verify file was written correctly
	if err := h.verifyFile(filePath, fileData, checksum); err != nil {
		_ = h.storage.Delete(filePath)
		respondError(w, http.StatusInternalServerError, "File verification failed", err)
		return
	}

	// Store metadata in Redis
	metadata := &models.FileMetadata{
		Filename:   filename,
		Version:    version,
		Checksum:   checksum,
		FilePath:   filePath,
		UploadedAt: time.Now(),
		Size:       header.Size,
	}

	if err := h.storeMetadata(ctx, filePath, metadata); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to store metadata", err)
		return
	}

	// Return success response
	respondJSON(w, http.StatusOK, models.UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		Metadata: metadata,
	})
}

// validateRequest validates the HTTP request method
func (h *UploadHandler) validateRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// parseUploadRequest parses and validates the upload request
func (h *UploadHandler) parseUploadRequest(w http.ResponseWriter, r *http.Request) (string, string, []byte, *multipart.FileHeader, error) {
	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Failed to parse form", err)
		return "", "", nil, nil, err
	}

	// Get form values
	filename := r.FormValue("filename")
	version := r.FormValue("version")

	if filename == "" || version == "" {
		respondError(w, http.StatusBadRequest, "filename and version are required", nil)
		return "", "", nil, nil, fmt.Errorf("missing filename or version")
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to get file from form", err)
		return "", "", nil, nil, err
	}
	defer file.Close()

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to read file", err)
		return "", "", nil, nil, err
	}

	return filename, version, fileData, header, nil
}

// acquireUploadLock acquires a distributed lock for the upload
// Returns lockKey and true if lock was acquired, false otherwise
func (h *UploadHandler) acquireUploadLock(w http.ResponseWriter, ctx context.Context, filename, version string) (string, bool) {
	lockKey := fmt.Sprintf("lock:upload:%s:%s", filename, version)
	acquired, err := h.redis.AcquireLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to acquire lock", err)
		return "", false
	}
	if !acquired {
		respondError(w, http.StatusConflict, "Upload already in progress for this file and version", nil)
		return "", false
	}
	return lockKey, true
}

// processFileUpload processes file upload with parallel checksum calculation and storage
func (h *UploadHandler) processFileUpload(filename, version string, fileData []byte) (string, string, error) {
	var checksum string
	var filePath string
	var writeErr error

	var wg sync.WaitGroup

	// Calculate checksum in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		hash := sha256.Sum256(fileData)
		checksum = hex.EncodeToString(hash[:])
	}()

	// Write file in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		filePath, writeErr = h.storage.Save(filename, version, fileData)
	}()

	wg.Wait()

	if writeErr != nil {
		return "", "", fmt.Errorf("failed to save file: %w", writeErr)
	}

	return checksum, filePath, nil
}

// verifyFile verifies that the file was written correctly
func (h *UploadHandler) verifyFile(filePath string, originalData []byte, expectedChecksum string) error {
	// Read back the file to verify it was written correctly
	verifiedData, err := h.storage.Get(filePath)
	if err != nil {
		return fmt.Errorf("file not readable: %w", err)
	}

	// Verify file size matches
	if len(verifiedData) != len(originalData) {
		return fmt.Errorf("size mismatch: expected %d, got %d", len(originalData), len(verifiedData))
	}

	// Verify checksum matches
	verifiedHash := sha256.Sum256(verifiedData)
	verifiedChecksum := hex.EncodeToString(verifiedHash[:])
	if verifiedChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, verifiedChecksum)
	}

	return nil
}

// storeMetadata stores metadata in Redis and handles rollback on failure
func (h *UploadHandler) storeMetadata(ctx context.Context, filePath string, metadata *models.FileMetadata) error {
	if err := h.redis.StoreFileMetadata(ctx, metadata); err != nil {
		// Try to clean up file if Redis storage fails
		_ = h.storage.Delete(filePath)
		return fmt.Errorf("failed to store metadata: %w", err)
	}
	return nil
}

// HandleHealth handles GET /health requests
func (h *UploadHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// Helper functions

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if data != nil {
		jsonData, _ := json.Marshal(data)
		w.Write(jsonData)
	}
}

func respondError(w http.ResponseWriter, statusCode int, message string, err error) {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
	}
	
	response := models.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: errorMsg,
	}
	
	respondJSON(w, statusCode, response)
}
