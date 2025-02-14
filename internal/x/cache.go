package x

import (
	"fmt"
	"os"
	"path/filepath"
)

type Cache interface {
	Get(key string) (string, bool)
	Set(key string, content string) error
}

// FileCache handles content caching
type FileCache struct {
	dir string
}

// NewFileCache creates a new cache instance
func NewFileCache(cacheDir string) (*FileCache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &FileCache{dir: cacheDir}, nil
}

// Get retrieves content from cache
func (c *FileCache) Get(key string) (string, bool) {
	path := filepath.Join(c.dir, key)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(content), true
}

// Set stores content in cache
func (c *FileCache) Set(key string, content string) error {
	path := filepath.Join(c.dir, key)
	return os.WriteFile(path, []byte(content), 0644)
}

func (c *FileCache) Clear() error {
	return os.RemoveAll(c.dir)
}
