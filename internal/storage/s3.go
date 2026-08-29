package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/priyanshuguptadev/job-board/internal/config"
)

// S3Storage provides S3-compatible object storage implementation of Storage interface.
type S3Storage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	defaultExpiry time.Duration
}

// Ensure S3Storage implements Storage interface.
var _ Storage = (*S3Storage)(nil)

// NewS3Storage initializes an S3Storage instance from configuration.
// It supports standard AWS S3 as well as custom S3-compatible endpoints (e.g. MinIO, Cloudflare R2, GCS).
func NewS3Storage(ctx context.Context, cfg config.S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3 bucket name is required")
	}

	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	}

	client := s3.NewFromConfig(awsCfg, s3Opts)
	presignClient := s3.NewPresignClient(client)

	expiry := time.Duration(cfg.PresignExpiryMinutes) * time.Minute
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	return &S3Storage{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		defaultExpiry: expiry,
	}, nil
}

// NewS3StorageWithClients creates an S3Storage instance with custom S3 and Presign clients (useful for testing).
func NewS3StorageWithClients(client *s3.Client, presignClient *s3.PresignClient, bucket string, defaultExpiry time.Duration) *S3Storage {
	if defaultExpiry <= 0 {
		defaultExpiry = 15 * time.Minute
	}
	return &S3Storage{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
		defaultExpiry: defaultExpiry,
	}
}

// Upload stores an object with the given key, content stream, size, and content type.
func (s *S3Storage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload object %q to s3: %w", key, err)
	}

	return nil
}

// GetPresignedURL generates a presigned download URL for an object with the given expiration duration.
// If expiry <= 0, the default configured expiration duration is used.
func (s *S3Storage) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = s.defaultExpiry
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	req, err := s.presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download url for %q: %w", key, err)
	}

	return req.URL, nil
}

// Delete removes an object from the S3 bucket.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object %q from s3: %w", key, err)
	}

	return nil
}

// Download retrieves the object stream, content type, and content length for the given key.
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	out, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to download object %q from s3: %w", key, err)
	}

	var contentType string
	if out.ContentType != nil {
		contentType = *out.ContentType
	}

	var contentLength int64
	if out.ContentLength != nil {
		contentLength = *out.ContentLength
	}

	return out.Body, contentType, contentLength, nil
}

// Exists checks if an object with the given key exists in the S3 bucket.
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.HeadObject(ctx, input)
	if err != nil {
		var notFound *types.NotFound
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &notFound) || errors.As(err, &noSuchKey) {
			return false, nil
		}

		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "404" {
				return false, nil
			}
		}

		// Also check for standard 404 response
		var responseError interface{ HTTPStatusCode() int }
		if errors.As(err, &responseError) && responseError.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}

		return false, fmt.Errorf("failed to check existence of object %q in s3: %w", key, err)
	}

	return true, nil
}

// Bucket returns the configured S3 bucket name.
func (s *S3Storage) Bucket() string {
	return s.bucket
}

// Client returns the underlying S3 client.
func (s *S3Storage) Client() *s3.Client {
	return s.client
}

// PresignClient returns the underlying S3 presign client.
func (s *S3Storage) PresignClient() *s3.PresignClient {
	return s.presignClient
}
