package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"control-plane/internal/models"
)

// StoreFileMetadata stores file metadata in Redis
// Key patterns:
//   - file:{filename}:{version} - Hash with metadata
//   - file:{filename}:versions - Sorted Set with versions
//   - file:{filename}:latest - String with latest version
func (c *Client) StoreFileMetadata(ctx context.Context, metadata *models.FileMetadata) error {
	// Create keys
	fileKey := fmt.Sprintf("file:%s:%s", metadata.Filename, metadata.Version)
	versionsKey := fmt.Sprintf("file:%s:versions", metadata.Filename)
	latestKey := fmt.Sprintf("file:%s:latest", metadata.Filename)

	// Serialize metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Use transaction for atomicity
	pipe := c.rdb.TxPipeline()

	// Store file metadata as hash
	pipe.HSet(ctx, fileKey, map[string]interface{}{
		"checksum":    metadata.Checksum,
		"version":     metadata.Version,
		"filepath":    metadata.FilePath,
		"uploaded_at": metadata.UploadedAt.Format(time.RFC3339),
		"size":        metadata.Size,
		"metadata":    string(metadataJSON),
	})

	// Add version to sorted set (use timestamp as score for ordering)
	pipe.ZAdd(ctx, versionsKey, redis.Z{
		Score:  float64(metadata.UploadedAt.Unix()),
		Member: metadata.Version,
	})

	// Update latest version
	pipe.Set(ctx, latestKey, metadata.Version, 0)

	// Execute transaction
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	return nil
}

// GetFileMetadata retrieves file metadata by filename and version
func (c *Client) GetFileMetadata(ctx context.Context, filename string, version string) (*models.FileMetadata, error) {
	fileKey := fmt.Sprintf("file:%s:%s", filename, version)

	// Get metadata hash
	result := c.rdb.HGetAll(ctx, fileKey)
	if result.Err() != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", result.Err())
	}

	if len(result.Val()) == 0 {
		return nil, fmt.Errorf("file not found: %s version %s", filename, version)
	}

	// Parse metadata
	metadataJSON := result.Val()["metadata"]
	var metadata models.FileMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// GetLatestVersion returns the latest version for a filename
func (c *Client) GetLatestVersion(ctx context.Context, filename string) (string, error) {
	latestKey := fmt.Sprintf("file:%s:latest", filename)

	version, err := c.rdb.Get(ctx, latestKey).Result()
	if err == redis.Nil {
		// Try to get from sorted set as fallback
		versionsKey := fmt.Sprintf("file:%s:versions", filename)
		versions, err := c.rdb.ZRevRange(ctx, versionsKey, 0, 0).Result()
		if err != nil || len(versions) == 0 {
			return "", fmt.Errorf("no versions found for file: %s", filename)
		}
		return versions[0], nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get latest version: %w", err)
	}

	return version, nil
}

// GetLatestFileMetadata retrieves metadata for the latest version of a file
func (c *Client) GetLatestFileMetadata(ctx context.Context, filename string) (*models.FileMetadata, error) {
	version, err := c.GetLatestVersion(ctx, filename)
	if err != nil {
		return nil, err
	}

	return c.GetFileMetadata(ctx, filename, version)
}

// GetAllVersions returns all versions for a filename (sorted, newest first)
func (c *Client) GetAllVersions(ctx context.Context, filename string) ([]string, error) {
	versionsKey := fmt.Sprintf("file:%s:versions", filename)

	versions, err := c.rdb.ZRevRange(ctx, versionsKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get versions: %w", err)
	}

	return versions, nil
}
