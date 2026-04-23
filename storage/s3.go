package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var ErrObjectNotFound = errors.New("s3 object not found")

type S3Storage struct {
	client         *s3.Client
	presignClient  *s3.PresignClient
	bucket         string
	uploadURLTTL   time.Duration
	downloadURLTTL time.Duration
}

func NewS3Storage(ctx context.Context, region, bucket, accessKey, secretKey, sessionToken string, uploadURLTTL, downloadURLTTL int) (*S3Storage, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("s3 bucket is required")
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if accessKey != "" && secretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	return &S3Storage{
		client:         client,
		presignClient:  s3.NewPresignClient(client),
		bucket:         bucket,
		uploadURLTTL:   time.Duration(uploadURLTTL) * time.Second,
		downloadURLTTL: time.Duration(downloadURLTTL) * time.Second,
	}, nil
}

func (s *S3Storage) PresignPutObject(ctx context.Context, objectKey, contentType string) (string, error) {
	if strings.TrimSpace(objectKey) == "" {
		return "", errors.New("object key is required")
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	req, err := s.presignClient.PresignPutObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = s.uploadURLTTL
	})
	if err != nil {
		return "", fmt.Errorf("presign put object: %w", err)
	}
	return req.URL, nil
}

func (s *S3Storage) PresignGetObject(ctx context.Context, objectKey string) (string, error) {
	if strings.TrimSpace(objectKey) == "" {
		return "", errors.New("object key is required")
	}
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = s.downloadURLTTL
	})
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}
	return req.URL, nil
}

func (s *S3Storage) EnsureObjectExists(ctx context.Context, objectKey string) error {
	if strings.TrimSpace(objectKey) == "" {
		return errors.New("object key is required")
	}

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
		return ErrObjectNotFound
	}
	if strings.Contains(strings.ToLower(err.Error()), "notfound") || strings.Contains(strings.ToLower(err.Error()), "status code: 404") {
		return ErrObjectNotFound
	}
	return fmt.Errorf("head object: %w", err)
}
