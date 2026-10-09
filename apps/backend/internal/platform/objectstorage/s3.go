// Package objectstorage holds the adapters of the shared/storage port
// (ADR 0065): the REG.RU S3-compatible client and the in-memory fake. No
// adapter builds a public URL — the private photos stream through the
// backend only.
package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// S3Storage stores private photos in an S3-compatible object store
// (REG.RU Object Storage, ADR 0005/0065): path-style addressing, no
// canned-ACL — the bucket stays private, every read goes through the
// backend.
type S3Storage struct {
	client *s3.Client
	bucket string
}

// NewS3Storage creates the S3 adapter. The ctx scopes the AWS config
// loading done during construction.
func NewS3Storage(ctx context.Context, endpoint, region, bucket, accessKey, secretKey string) (*S3Storage, error) {
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
		// Рег.ру Object Storage has no virtual-host domains (research
		// #1218): path-style is mandatory.
		o.UsePathStyle = true
	})

	return &S3Storage{client: client, bucket: bucket}, nil
}

// Put stores the object under the key. The content type is the
// magic-byte-determined value from the upload seam, never a client header.
func (s *S3Storage) Put(ctx context.Context, key string, data io.Reader, contentType string, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          data,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

// Open returns the object's body, byte size and stored content type; a
// missing key is storage.ErrNotFound.
func (s *S3Storage) Open(ctx context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, 0, "", storage.ErrNotFound
		}
		return nil, 0, "", fmt.Errorf("open object: %w", err)
	}
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return out.Body, size, contentType, nil
}

// Delete removes the object with the given key. S3 delete of an absent key
// succeeds — the best-effort orphan cleanup never fails on one.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

// HeadBucket checks whether the configured bucket exists and is reachable;
// the wire layer calls it once at startup through a type assertion.
func (s *S3Storage) HeadBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("head bucket: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var respErr *awshttp.ResponseError
	return errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound
}
