package models

import "time"

// FileMetadata represents metadata for an uploaded file
type FileMetadata struct {
	Filename   string    `json:"filename"`
	Version    string    `json:"version"`
	Checksum   string    `json:"checksum"`
	FilePath   string    `json:"filepath"`
	UploadedAt time.Time `json:"uploaded_at"`
	Size       int64     `json:"size"`
}

// UploadRequest represents the request body for file upload
type UploadRequest struct {
	Filename string `json:"filename"`
	Version  string `json:"version"`
}

// UploadResponse represents the response after successful upload
type UploadResponse struct {
	Success   bool          `json:"success"`
	Message   string        `json:"message"`
	Metadata  *FileMetadata `json:"metadata,omitempty"`
}

// FileInfoResponse represents file metadata response
type FileInfoResponse struct {
	Filename   string    `json:"filename"`
	Version    string    `json:"version"`
	Checksum   string    `json:"checksum"`
	UploadedAt time.Time `json:"uploaded_at"`
	Size       int64     `json:"size"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
