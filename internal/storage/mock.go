package storage

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	SaveFunc    func(filename string, version string, data []byte) (string, error)
	GetFunc     func(filepath string) ([]byte, error)
	DeleteFunc  func(filepath string) error
	GetPathFunc func(filename string, version string) string
}

func (m *MockStorage) Save(filename string, version string, data []byte) (string, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(filename, version, data)
	}
	return "", nil
}

func (m *MockStorage) Get(filepath string) ([]byte, error) {
	if m.GetFunc != nil {
		return m.GetFunc(filepath)
	}
	return nil, nil
}

func (m *MockStorage) Delete(filepath string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(filepath)
	}
	return nil
}

func (m *MockStorage) GetPath(filename string, version string) string {
	if m.GetPathFunc != nil {
		return m.GetPathFunc(filename, version)
	}
	return ""
}
