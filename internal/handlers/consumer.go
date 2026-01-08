package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"

	"control-plane/internal/config"
	"control-plane/internal/models"
	"control-plane/internal/redis"
	"control-plane/internal/storage"
)

// ConsumerHandler handles file retrieval requests
type ConsumerHandler struct {
	storage storage.Storage
	redis   *redis.Client
	config  *config.Config
}

// NewConsumerHandler creates a new consumer handler
func NewConsumerHandler(storage storage.Storage, redisClient *redis.Client, cfg *config.Config) *ConsumerHandler {
	return &ConsumerHandler{
		storage: storage,
		redis:   redisClient,
		config:  cfg,
	}
}

// HandleGetFile handles GET /file/{filename} - returns latest version
func (h *ConsumerHandler) HandleGetFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	if filename == "" {
		respondError(w, http.StatusBadRequest, "filename is required", nil)
		return
	}

	ctx := r.Context()

	// Get latest version metadata
	metadata, err := h.redis.GetLatestFileMetadata(ctx, filename)
	if err != nil {
		respondError(w, http.StatusNotFound, "File not found", err)
		return
	}

	// Get file data
	fileData, err := h.storage.Get(metadata.FilePath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to read file", err)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(metadata.Filename)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))
	w.Header().Set("X-File-Version", metadata.Version)
	w.Header().Set("X-File-Checksum", metadata.Checksum)

	// Write file
	w.WriteHeader(http.StatusOK)
	w.Write(fileData)
}

// HandleGetFileVersion handles GET /file/{filename}/version/{version}
func (h *ConsumerHandler) HandleGetFileVersion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]
	version := vars["version"]

	if filename == "" || version == "" {
		respondError(w, http.StatusBadRequest, "filename and version are required", nil)
		return
	}

	ctx := r.Context()

	// Get specific version metadata
	metadata, err := h.redis.GetFileMetadata(ctx, filename, version)
	if err != nil {
		respondError(w, http.StatusNotFound, "File version not found", err)
		return
	}

	// Get file data
	fileData, err := h.storage.Get(metadata.FilePath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to read file", err)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(metadata.Filename)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))
	w.Header().Set("X-File-Version", metadata.Version)
	w.Header().Set("X-File-Checksum", metadata.Checksum)

	// Write file
	w.WriteHeader(http.StatusOK)
	w.Write(fileData)
}

// HandleGetFileInfo handles GET /file/{filename}/info - returns metadata
func (h *ConsumerHandler) HandleGetFileInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	if filename == "" {
		respondError(w, http.StatusBadRequest, "filename is required", nil)
		return
	}

	ctx := r.Context()

	// Get latest version metadata
	metadata, err := h.redis.GetLatestFileMetadata(ctx, filename)
	if err != nil {
		respondError(w, http.StatusNotFound, "File not found", err)
		return
	}

	// Return metadata
	response := models.FileInfoResponse{
		Filename:   metadata.Filename,
		Version:    metadata.Version,
		Checksum:   metadata.Checksum,
		UploadedAt: metadata.UploadedAt,
		Size:       metadata.Size,
	}

	respondJSON(w, http.StatusOK, response)
}

// HandleHealth handles GET /health requests
func (h *ConsumerHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
