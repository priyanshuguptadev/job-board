package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3Storage_Validation(t *testing.T) {
	ctx := context.Background()

	// Missing bucket name
	_, err := NewS3Storage(ctx, config.S3Config{
		Bucket: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Valid AWS configuration
	s3Store, err := NewS3Storage(ctx, config.S3Config{
		Region:               "us-east-1",
		Bucket:               "my-jobboard-resumes",
		AccessKeyID:          "test-access-key",
		SecretAccessKey:      "test-secret-key",
		PresignExpiryMinutes: 20,
	})
	require.NoError(t, err)
	require.NotNil(t, s3Store)
	assert.Equal(t, "my-jobboard-resumes", s3Store.Bucket())
	assert.NotNil(t, s3Store.Client())
	assert.NotNil(t, s3Store.PresignClient())
}

func TestNewS3Storage_Providers(t *testing.T) {
	ctx := context.Background()

	providers := []struct {
		name string
		cfg  config.S3Config
	}{
		{
			name: "AWS S3 standard",
			cfg: config.S3Config{
				Region:               "us-west-2",
				Bucket:               "aws-bucket",
				AccessKeyID:          "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey:      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				ForcePathStyle:       false,
				PresignExpiryMinutes: 15,
			},
		},
		{
			name: "MinIO Local",
			cfg: config.S3Config{
				Endpoint:             "http://localhost:9000",
				Region:               "us-east-1",
				Bucket:               "minio-bucket",
				AccessKeyID:          "minioadmin",
				SecretAccessKey:      "minioadmin",
				ForcePathStyle:       true,
				PresignExpiryMinutes: 30,
			},
		},
		{
			name: "Cloudflare R2",
			cfg: config.S3Config{
				Endpoint:             "https://abc123xyz.r2.cloudflarestorage.com",
				Region:               "auto",
				Bucket:               "r2-resumes",
				AccessKeyID:          "r2-access-key",
				SecretAccessKey:      "r2-secret-key",
				ForcePathStyle:       false,
				PresignExpiryMinutes: 10,
			},
		},
		{
			name: "Google Cloud Storage S3 API",
			cfg: config.S3Config{
				Endpoint:             "https://storage.googleapis.com",
				Region:               "auto",
				Bucket:               "gcs-bucket",
				AccessKeyID:          "gcs-access-key",
				SecretAccessKey:      "gcs-secret-key",
				ForcePathStyle:       false,
				PresignExpiryMinutes: 15,
			},
		},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			store, err := NewS3Storage(ctx, p.cfg)
			require.NoError(t, err)
			assert.Equal(t, p.cfg.Bucket, store.Bucket())
			assert.NotNil(t, store.Client())
			assert.NotNil(t, store.PresignClient())
		})
	}
}

// mockS3Server implements a minimal in-memory HTTP server handling S3 REST operations.
type mockS3Server struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
}

func newMockS3Server() (*httptest.Server, *mockS3Server) {
	mock := &mockS3Server{
		objects: make(map[string][]byte),
		types:   make(map[string]string),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.mu.Lock()
		defer mock.mu.Unlock()

		path := r.URL.Path

		switch r.Method {
		case http.MethodPut:
			data, _ := io.ReadAll(r.Body)
			mock.objects[path] = data
			mock.types[path] = r.Header.Get("Content-Type")
			w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
			w.WriteHeader(http.StatusOK)

		case http.MethodGet:
			data, ok := mock.objects[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
				return
			}
			w.Header().Set("Content-Type", mock.types[path])
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)

		case http.MethodHead:
			_, ok := mock.objects[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", mock.types[path])
			w.WriteHeader(http.StatusOK)

		case http.MethodDelete:
			delete(mock.objects, path)
			delete(mock.types, path)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	return ts, mock
}

func TestS3Storage_Operations(t *testing.T) {
	ts, _ := newMockS3Server()
	defer ts.Close()

	ctx := context.Background()
	bucket := "test-bucket"

	cfg := config.S3Config{
		Endpoint:             ts.URL,
		Region:               "us-east-1",
		Bucket:               bucket,
		AccessKeyID:          "test-key",
		SecretAccessKey:      "test-secret",
		ForcePathStyle:       true,
		PresignExpiryMinutes: 15,
	}

	storage, err := NewS3Storage(ctx, cfg)
	require.NoError(t, err)

	key := "resumes/job1/app1/resume.pdf"
	content := []byte("%PDF-1.4\nTest Resume Content")

	// 1. Exists before upload (should be false)
	exists, err := storage.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	// 2. Upload
	err = storage.Upload(ctx, key, bytes.NewReader(content), int64(len(content)), "application/pdf")
	require.NoError(t, err)

	// 3. Exists after upload (should be true)
	exists, err = storage.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	// 4. Download
	rc, ct, size, err := storage.Download(ctx, key)
	require.NoError(t, err)
	require.NotNil(t, rc)
	assert.Equal(t, "application/pdf", ct)

	downloaded, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)
	_ = rc.Close()
	_ = size

	// 5. Presigned URL generation
	presignedURL, err := storage.GetPresignedURL(ctx, key, 10*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, presignedURL)
	assert.Contains(t, presignedURL, key)
	assert.Contains(t, presignedURL, "X-Amz-Signature")
	assert.Contains(t, presignedURL, "X-Amz-Expires")

	// 6. Delete
	err = storage.Delete(ctx, key)
	require.NoError(t, err)

	// 7. Exists after delete (should be false)
	exists, err = storage.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)
}
