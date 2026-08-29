package storage

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStorage(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	// Initial assertions
	assert.Equal(t, 0, store.Count())

	// Exists on missing key
	exists, err := store.Exists(ctx, "non-existent-key")
	require.NoError(t, err)
	assert.False(t, exists)

	// Download on missing key
	rc, ct, size, err := store.Download(ctx, "non-existent-key")
	require.Error(t, err)
	assert.Nil(t, rc)
	assert.Empty(t, ct)
	assert.Zero(t, size)

	// Presigned URL on missing key
	pURL, err := store.GetPresignedURL(ctx, "non-existent-key", 10*time.Minute)
	require.Error(t, err)
	assert.Empty(t, pURL)

	// Upload object
	content := []byte("resume binary payload data")
	err = store.Upload(ctx, "resumes/job1/app1/resume.pdf", bytes.NewReader(content), int64(len(content)), "application/pdf")
	require.NoError(t, err)
	assert.Equal(t, 1, store.Count())

	// Check Exists
	exists, err = store.Exists(ctx, "resumes/job1/app1/resume.pdf")
	require.NoError(t, err)
	assert.True(t, exists)

	// Download object
	rc, ct, size, err = store.Download(ctx, "resumes/job1/app1/resume.pdf")
	require.NoError(t, err)
	require.NotNil(t, rc)
	assert.Equal(t, "application/pdf", ct)
	assert.Equal(t, int64(len(content)), size)

	downloadedBytes, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, downloadedBytes)
	_ = rc.Close()

	// Presigned URL
	store.SetBaseURL("https://custom-s3.example.com")
	pURL, err = store.GetPresignedURL(ctx, "resumes/job1/app1/resume.pdf", 5*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, pURL, "https://custom-s3.example.com/resumes/job1/app1/resume.pdf")
	assert.Contains(t, pURL, "expires=")

	// Test GetObject
	obj, ok := store.GetObject("resumes/job1/app1/resume.pdf")
	require.True(t, ok)
	assert.Equal(t, content, obj.Data)
	assert.Equal(t, "application/pdf", obj.ContentType)

	// Delete object
	err = store.Delete(ctx, "resumes/job1/app1/resume.pdf")
	require.NoError(t, err)
	assert.Equal(t, 0, store.Count())

	exists, err = store.Exists(ctx, "resumes/job1/app1/resume.pdf")
	require.NoError(t, err)
	assert.False(t, exists)

	// Reset
	_ = store.Upload(ctx, "key1", bytes.NewReader([]byte("test")), 4, "text/plain")
	assert.Equal(t, 1, store.Count())
	store.Reset()
	assert.Equal(t, 0, store.Count())
}

func TestMemoryStorage_Concurrency(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	var wg sync.WaitGroup
	workers := 20
	opsPerWorker := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				key := "key"
				data := []byte("concurrency-test-data")
				_ = store.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), "text/plain")
				_, _ = store.Exists(ctx, key)
				rc, _, _, err := store.Download(ctx, key)
				if err == nil && rc != nil {
					_, _ = io.ReadAll(rc)
					_ = rc.Close()
				}
				_, _ = store.GetPresignedURL(ctx, key, time.Minute)
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryStorage_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := NewMemoryStorage()

	err := store.Upload(ctx, "key", bytes.NewReader([]byte("data")), 4, "text/plain")
	assert.ErrorIs(t, err, context.Canceled)

	_, err = store.Exists(ctx, "key")
	assert.ErrorIs(t, err, context.Canceled)

	_, _, _, err = store.Download(ctx, "key")
	assert.ErrorIs(t, err, context.Canceled)

	_, err = store.GetPresignedURL(ctx, "key", time.Minute)
	assert.ErrorIs(t, err, context.Canceled)

	err = store.Delete(ctx, "key")
	assert.ErrorIs(t, err, context.Canceled)
}
