package storage

import (
	"context"
	"io"
	"time"
)

// Storage defines the interface for object storage operations.
type Storage interface {
	// Upload stores an object with the given key, content stream, size, and content type.
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error

	// GetPresignedURL generates a presigned download URL for an object with the given expiration duration.
	// If expiry is <= 0, the default configured expiration duration is used.
	GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)

	// Delete removes an object with the given key from storage.
	Delete(ctx context.Context, key string) error

	// Download retrieves the object stream, content type, and content length for the given key.
	Download(ctx context.Context, key string) (io.ReadCloser, string, int64, error)

	// Exists checks if an object with the given key exists in storage.
	Exists(ctx context.Context, key string) (bool, error)
}
