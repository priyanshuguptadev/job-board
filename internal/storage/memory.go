package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"
)

// StoredObject represents an object in MemoryStorage.
type StoredObject struct {
	Key         string
	Data        []byte
	ContentType string
	Size        int64
	UpdatedAt   time.Time
}

// MemoryStorage is an in-memory implementation of the Storage interface,
// useful for unit and integration testing without needing external S3 infrastructure.
type MemoryStorage struct {
	mu            sync.RWMutex
	objects       map[string]*StoredObject
	defaultExpiry time.Duration
	baseURL       string
}

// Ensure MemoryStorage implements Storage interface.
var _ Storage = (*MemoryStorage)(nil)

// NewMemoryStorage creates a new in-memory storage instance.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		objects:       make(map[string]*StoredObject),
		defaultExpiry: 15 * time.Minute,
		baseURL:       "https://mock-storage.local",
	}
}

// SetBaseURL overrides the base URL used when generating mock presigned URLs.
func (m *MemoryStorage) SetBaseURL(baseURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseURL = baseURL
}

// Upload stores an object in memory.
func (m *MemoryStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.objects[key] = &StoredObject{
		Key:         key,
		Data:        data,
		ContentType: contentType,
		Size:        int64(len(data)),
		UpdatedAt:   time.Now().UTC(),
	}

	return nil
}

// GetPresignedURL simulates generating a presigned download URL for an in-memory object.
func (m *MemoryStorage) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if expiry <= 0 {
		expiry = m.defaultExpiry
	}

	m.mu.RLock()
	obj, exists := m.objects[key]
	baseURL := m.baseURL
	m.mu.RUnlock()

	if !exists {
		return "", errors.New("object not found")
	}

	expiresAt := time.Now().UTC().Add(expiry).Unix()
	cleanKey := strings.TrimPrefix(obj.Key, "/")
	presignedURL := fmt.Sprintf("%s/%s?expires=%d&content_type=%s", baseURL, cleanKey, expiresAt, url.QueryEscape(obj.ContentType))
	return presignedURL, nil
}

// Delete removes an object from in-memory storage.
func (m *MemoryStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.objects, key)
	return nil
}

// Download retrieves an object from in-memory storage as a ReadCloser.
func (m *MemoryStorage) Download(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", 0, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	obj, exists := m.objects[key]
	if !exists {
		return nil, "", 0, errors.New("object not found")
	}

	dataCopy := make([]byte, len(obj.Data))
	copy(dataCopy, obj.Data)

	return io.NopCloser(bytes.NewReader(dataCopy)), obj.ContentType, obj.Size, nil
}

// Exists checks if an object exists in in-memory storage.
func (m *MemoryStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.objects[key]
	return exists, nil
}

// GetObject retrieves the raw StoredObject directly for assertions in tests.
func (m *MemoryStorage) GetObject(key string) (*StoredObject, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	obj, exists := m.objects[key]
	if !exists {
		return nil, false
	}
	return obj, true
}

// Count returns the number of objects stored.
func (m *MemoryStorage) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.objects)
}

// Reset clears all stored objects.
func (m *MemoryStorage) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects = make(map[string]*StoredObject)
}
