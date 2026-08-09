package platform

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Storage wraps Supabase Storage's S3-compatible API. Only the API holds
// storage credentials (AD-2); browsers load objects via the public CDN URL.
type Storage struct {
	client     *s3.Client
	bucket     string
	publicBase string // SUPABASE_URL, base for public object URLs
}

// NewStorage builds the S3 client for Supabase Storage. Both checksum modes
// MUST stay `when_required`: the SDK's flexible-checksum defaults send
// headers Supabase S3 does not understand and requests fail with
// SignatureDoesNotMatch.
func NewStorage(ctx context.Context, cfg *Config) (*Storage, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.StorageS3Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.StorageS3AccessKey, cfg.StorageS3SecretKey, "")),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.StorageS3Endpoint)
		o.UsePathStyle = true
	})
	return &Storage{client: client, bucket: cfg.StorageBucket, publicBase: cfg.SupabaseURL}, nil
}

// Upload writes one object to the public bucket.
func (s *Storage) Upload(ctx context.Context, path, contentType string, body io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", path, err)
	}
	return nil
}

// Delete removes one object. S3 deletes are idempotent: deleting a missing
// key succeeds.
func (s *Storage) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", path, err)
	}
	return nil
}

// PublicURL renders the CDN URL browsers load the object from.
func (s *Storage) PublicURL(path string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.publicBase, s.bucket, path)
}
