package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage uploads property photos to an S3-compatible object store.
type S3Storage struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

// NewS3Storage creates an S3-compatible storage adapter. The ctx scopes the
// AWS config loading done during construction.
func NewS3Storage(
	ctx context.Context, endpoint, region, bucket, accessKey, secretKey, publicBaseURL string, pathStyle bool,
) (*S3Storage, error) {
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

	endpoint = strings.TrimRight(endpoint, "/")

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = pathStyle
	})

	return &S3Storage{
		client:        client,
		bucket:        bucket,
		publicBaseURL: publicBaseURL,
	}, nil
}

// Upload stores the object under the given key and returns its public URL.
func (s *S3Storage) Upload(ctx context.Context, key, contentType string, size int64, data io.Reader) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          data,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		ACL:           types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	publicURL, err := url.JoinPath(s.publicBaseURL, key)
	if err != nil {
		return "", fmt.Errorf("build public url: %w", err)
	}
	return publicURL, nil
}

// Delete removes the object with the given key from S3.
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

// HeadBucket checks whether the configured bucket exists and is reachable.
func (s *S3Storage) HeadBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("head bucket: %w", err)
	}
	return nil
}
