package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage_Save(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	testData := []byte("test file content")
	filename := "testfile"
	version := "1.0.0"

	// Test Save
	filePath, err := storage.Save(filename, version, testData)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, filename, version)
	if filePath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, filePath)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("File was not created at %s", filePath)
	}
}

func TestLocalStorage_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	testData := []byte("test file content")
	filename := "testfile"
	version := "1.0.0"

	// Save file first
	filePath, err := storage.Save(filename, version, testData)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Test Get
	retrievedData, err := storage.Get(filePath)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrievedData) != string(testData) {
		t.Errorf("Expected %s, got %s", string(testData), string(retrievedData))
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	testData := []byte("test file content")
	filename := "testfile"
	version := "1.0.0"

	// Save file first
	filePath, err := storage.Save(filename, version, testData)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Test Delete
	err = storage.Delete(filePath)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("File still exists after delete")
	}
}

func TestLocalStorage_GetPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	filename := "testfile"
	version := "1.0.0"

	path := storage.GetPath(filename, version)
	expectedPath := filepath.Join(tmpDir, filename, version)

	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}
}

func TestLocalStorage_Get_Nonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Try to get nonexistent file
	_, err = storage.Get("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}
