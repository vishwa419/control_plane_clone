package storage

// Storage defines the interface for file storage operations
type Storage interface {
	// Save stores a file and returns the file path
	Save(filename string, version string, data []byte) (string, error)
	// Get retrieves a file by its path
	Get(filepath string) ([]byte, error)
	// Delete removes a file by its path
	Delete(filepath string) error
	// GetPath returns the storage path for a given filename and version
	GetPath(filename string, version string) string
}
