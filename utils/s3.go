package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type S3Service struct {
	bucket      string
	endpoint    string
	uploadTTL   time.Duration
	downloadTTL time.Duration
	httpClient  *http.Client
}

type S3InitParams struct {
	Region             string
	Bucket             string
	AccessKey          string
	SecretKey          string
	Endpoint           string
	UsePathStyle       bool
	UploadTTLSeconds   int
	DownloadTTLSeconds int
}

func NewS3Service(_ context.Context, p S3InitParams) (*S3Service, error) {
	if p.Bucket == "" || p.Endpoint == "" {
		return nil, fmt.Errorf("s3 bucket and endpoint are required")
	}
	uploadTTL := time.Duration(p.UploadTTLSeconds) * time.Second
	if uploadTTL == 0 {
		uploadTTL = 15 * time.Minute
	}
	downloadTTL := time.Duration(p.DownloadTTLSeconds) * time.Second
	if downloadTTL == 0 {
		downloadTTL = 15 * time.Minute
	}
	return &S3Service{bucket: p.Bucket, endpoint: strings.TrimRight(p.Endpoint, "/"), uploadTTL: uploadTTL, downloadTTL: downloadTTL, httpClient: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (s *S3Service) BuildObjectKey(fileName string) string {
	clean := path.Base(strings.TrimSpace(fileName))
	if clean == "." || clean == "/" || clean == "" {
		clean = fmt.Sprintf("creative-%d.bin", time.Now().UnixNano())
	}
	return fmt.Sprintf("creatives/%d/%s", time.Now().UnixNano(), clean)
}

func (s *S3Service) objectURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, strings.TrimLeft(key, "/"))
}

func (s *S3Service) PresignPutObject(_ context.Context, key, _ string) (string, int, error) {
	// For S3-compatible gateways with direct PUT support.
	return s.objectURL(key), int(s.uploadTTL.Seconds()), nil
}

func (s *S3Service) PresignGetObject(_ context.Context, key string) (string, int, error) {
	return s.objectURL(key), int(s.downloadTTL.Seconds()), nil
}

func (s *S3Service) ObjectExists(ctx context.Context, key string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.objectURL(key), nil)
	if err != nil {
		return false, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

func (s *S3Service) GetObject(ctx context.Context, key string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(key), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, "", fmt.Errorf("s3 get status: %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return resp.Body, contentType, nil
}

func ParseS3KeyFromPath(p string) string {
	u, err := url.Parse(strings.TrimSpace(p))
	if err == nil && u.Scheme != "" {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) > 1 {
			return strings.Join(parts[1:], "/")
		}
		return strings.TrimPrefix(u.Path, "/")
	}
	return strings.TrimPrefix(strings.TrimSpace(p), "/")
}
