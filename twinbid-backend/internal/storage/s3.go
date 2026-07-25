package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"twinbid-backend/internal/config"
)

type S3Storage struct {
	client *s3.Client
	bucket string
}

type ObjectMetadata struct {
	ContentType   string
	ContentLength int64
	ETag          string
	LastModified  *time.Time
}

type Object struct {
	ObjectMetadata
	Body io.ReadCloser
}

func NewS3(ctx context.Context, cfg config.S3Config) (*S3Storage, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		options.UsePathStyle = cfg.UsePathStyle
	})
	storage := &S3Storage{client: client, bucket: cfg.Bucket}
	if err := storage.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *S3Storage) Put(ctx context.Context, key, contentType string, body io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3Storage) Get(ctx context.Context, key string) (*Object, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return &Object{
		ObjectMetadata: ObjectMetadata{
			ContentType:   aws.ToString(output.ContentType),
			ContentLength: aws.ToInt64(output.ContentLength),
			ETag:          aws.ToString(output.ETag),
			LastModified:  output.LastModified,
		},
		Body: output.Body,
	}, nil
}

func (s *S3Storage) Head(ctx context.Context, key string) (ObjectMetadata, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectMetadata{}, err
	}
	return ObjectMetadata{
		ContentType:   aws.ToString(output.ContentType),
		ContentLength: aws.ToInt64(output.ContentLength),
		ETag:          aws.ToString(output.ETag),
		LastModified:  output.LastModified,
	}, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchBucket":
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") || strings.Contains(message, "nosuchkey")
}

func (s *S3Storage) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
		ACL:    types.BucketCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("create s3 bucket %s: %w", s.bucket, err)
	}
	return nil
}
