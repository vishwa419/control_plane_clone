package redis

import (
	"context"
	"testing"
	"time"

	"control-plane/internal/models"
)

// TestRedisClient requires a running Redis instance
// Skip if Redis is not available
func TestRedisClient_StoreAndRetrieve(t *testing.T) {
	// Try to connect to Redis (default localhost:6379)
	client, err := NewClient("localhost", "6379", "")
	if err != nil {
		t.Fatalf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test data
	metadata := &models.FileMetadata{
		Filename:   "test-file",
		Version:    "1.0.0",
		Checksum:   "abc123def456",
		FilePath:   "/app/files/test-file/1.0.0",
		UploadedAt: time.Now(),
		Size:       1024,
	}

	// Test StoreFileMetadata
	err = client.StoreFileMetadata(ctx, metadata)
	if err != nil {
		t.Fatalf("StoreFileMetadata failed: %v", err)
	}

	// Test GetFileMetadata
	retrieved, err := client.GetFileMetadata(ctx, "test-file", "1.0.0")
	if err != nil {
		t.Fatalf("GetFileMetadata failed: %v", err)
	}

	if retrieved.Filename != metadata.Filename {
		t.Errorf("Expected filename %s, got %s", metadata.Filename, retrieved.Filename)
	}
	if retrieved.Version != metadata.Version {
		t.Errorf("Expected version %s, got %s", metadata.Version, retrieved.Version)
	}
	if retrieved.Checksum != metadata.Checksum {
		t.Errorf("Expected checksum %s, got %s", metadata.Checksum, retrieved.Checksum)
	}
}

func TestRedisClient_LatestVersion(t *testing.T) {
	client, err := NewClient("localhost", "6379", "")
	if err != nil {
		t.Fatalf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Store v1.0.0
	metadata1 := &models.FileMetadata{
		Filename:   "version-test",
		Version:    "1.0.0",
		Checksum:   "checksum1",
		FilePath:   "/app/files/version-test/1.0.0",
		UploadedAt: time.Now(),
		Size:       100,
	}
	err = client.StoreFileMetadata(ctx, metadata1)
	if err != nil {
		t.Fatalf("StoreFileMetadata v1.0.0 failed: %v", err)
	}

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Store v2.0.0
	metadata2 := &models.FileMetadata{
		Filename:   "version-test",
		Version:    "2.0.0",
		Checksum:   "checksum2",
		FilePath:   "/app/files/version-test/2.0.0",
		UploadedAt: time.Now(),
		Size:       200,
	}
	err = client.StoreFileMetadata(ctx, metadata2)
	if err != nil {
		t.Fatalf("StoreFileMetadata v2.0.0 failed: %v", err)
	}

	// Test GetLatestVersion
	latestVersion, err := client.GetLatestVersion(ctx, "version-test")
	if err != nil {
		t.Fatalf("GetLatestVersion failed: %v", err)
	}

	if latestVersion != "2.0.0" {
		t.Errorf("Expected latest version 2.0.0, got %s", latestVersion)
	}

	// Test GetLatestFileMetadata
	latestMetadata, err := client.GetLatestFileMetadata(ctx, "version-test")
	if err != nil {
		t.Fatalf("GetLatestFileMetadata failed: %v", err)
	}

	if latestMetadata.Version != "2.0.0" {
		t.Errorf("Expected latest version 2.0.0, got %s", latestMetadata.Version)
	}
}

func TestRedisClient_GetAllVersions(t *testing.T) {
	client, err := NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Store multiple versions
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for i, v := range versions {
		metadata := &models.FileMetadata{
			Filename:   "multi-version",
			Version:    v,
			Checksum:   "checksum" + v,
			FilePath:   "/app/files/multi-version/" + v,
			UploadedAt: time.Now().Add(time.Duration(i) * time.Second),
			Size:       int64(100 * (i + 1)),
		}
		err = client.StoreFileMetadata(ctx, metadata)
		if err != nil {
			t.Fatalf("StoreFileMetadata %s failed: %v", v, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Test GetAllVersions
	allVersions, err := client.GetAllVersions(ctx, "multi-version")
	if err != nil {
		t.Fatalf("GetAllVersions failed: %v", err)
	}

	if len(allVersions) != len(versions) {
		t.Errorf("Expected %d versions, got %d", len(versions), len(allVersions))
	}

	// Should be sorted newest first
	if allVersions[0] != "2.0.0" {
		t.Errorf("Expected newest version 2.0.0, got %s", allVersions[0])
	}
}

func TestRedisClient_NonexistentFile(t *testing.T) {
	client, err := NewClient("localhost", "6379", "")
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test GetFileMetadata for nonexistent file
	_, err = client.GetFileMetadata(ctx, "nonexistent-file", "1.0.0")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}

	// Test GetLatestVersion for nonexistent file
	_, err = client.GetLatestVersion(ctx, "nonexistent-file")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}
