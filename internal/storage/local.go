package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalStorage implements Storage interface using local filesystem
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new LocalStorage instance
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: basePath,
	}, nil
}

// GetPath returns the storage path for a given filename and version
func (ls *LocalStorage) GetPath(filename string, version string) string {
	// Structure: files/{filename}/{version}
	return filepath.Join(ls.basePath, filename, version)
}

// Save stores a file and returns the file path
func (ls *LocalStorage) Save(filename string, version string, data []byte) (string, error) {
	filePath := ls.GetPath(filename, version)

	// Create directory structure
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}


	// Write file (will overwrite existing file)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

// Get retrieves a file by its path
func (ls *LocalStorage) Get(filepath string) ([]byte, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// Delete removes a file by its path
func (ls *LocalStorage) Delete(filepath string) error {
	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
