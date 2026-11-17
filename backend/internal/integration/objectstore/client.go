package objectstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// Client is the narrow S3-compatible surface required by PM artifact uploads.
// Bucket creation and public ACL changes remain deployment concerns.
type Client struct {
	client             *minio.Client
	bucket             string
	readinessKeyPrefix string
	readinessSequence  atomic.Uint64
}

func New(cfg config.ObjectStorage) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" || strings.TrimSpace(cfg.Bucket) == "" ||
		strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("object storage configuration is incomplete")
	}
	bucketLookup := minio.BucketLookupAuto
	if strings.Contains(strings.ToLower(endpoint), "myqcloud.com") {
		bucketLookup = minio.BucketLookupDNS
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		Region:       strings.TrimSpace(cfg.Region),
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize object storage: %w", err)
	}
	probeID := make([]byte, 16)
	if _, err := rand.Read(probeID); err != nil {
		return nil, fmt.Errorf("initialize object storage readiness probe: %w", err)
	}
	readinessKeyPrefix := path.Join(
		strings.Trim(strings.TrimSpace(cfg.GlobalPrefix), "/"),
		// Workspace IDs are positive database identifiers. The reserved zero scope
		// keeps the probe tenant-neutral while exercising the same IAM namespace
		// and path shape as every PM-owned artifact.
		"workspace", "0", ".pm-health", "object-storage-ready-"+hex.EncodeToString(probeID),
	)
	return &Client{
		client: client, bucket: strings.TrimSpace(cfg.Bucket), readinessKeyPrefix: readinessKeyPrefix,
	}, nil
}

func (c *Client) nextReadinessKey() string {
	return fmt.Sprintf("%s-%016x", c.readinessKeyPrefix, c.readinessSequence.Add(1))
}

func (c *Client) PutObject(
	ctx context.Context,
	objectKey string,
	body io.Reader,
	size int64,
	contentType string,
) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("object storage is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	_, err := c.client.PutObject(ctx, c.bucket, objectKey, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// CheckReady verifies the shared workspace IAM namespace used by artifact delivery.
// Cleanup is best-effort because immutable bucket policies may intentionally
// deny deletes while still supporting the application's required operations.
func (c *Client) CheckReady(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("object storage is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check object storage bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("object storage bucket does not exist")
	}
	probeKey := c.nextReadinessKey()
	payload := []byte("pm-object-storage-ready\n")
	if _, err := c.client.PutObject(ctx, c.bucket, probeKey, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{
		ContentType: "text/plain; charset=utf-8",
	}); err != nil {
		return fmt.Errorf("write object storage readiness probe: %w", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cleanupCancel()
		_ = c.client.RemoveObject(cleanupCtx, c.bucket, probeKey, minio.RemoveObjectOptions{})
	}()
	object, err := c.client.GetObject(ctx, c.bucket, probeKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("read object storage readiness probe: %w", err)
	}
	readBack, readErr := io.ReadAll(io.LimitReader(object, int64(len(payload)+1)))
	closeErr := object.Close()
	if readErr != nil {
		return fmt.Errorf("read object storage readiness probe: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close object storage readiness probe: %w", closeErr)
	}
	if !bytes.Equal(readBack, payload) {
		return fmt.Errorf("object storage readiness probe content mismatch")
	}
	return nil
}

// GetObject opens an object for streaming through an authenticated PM API.
// The request context controls cancellation for the full download lifetime.
func (c *Client) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("object storage is not configured")
	}
	return c.client.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
}

func (c *Client) RemoveObject(ctx context.Context, objectKey string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("object storage is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	return c.client.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{})
}

// PublicURL returns the stable browser-facing URL stored in the defect record.
// A deployment may supply a CDN/base URL; otherwise the original virtual-hosted
// bucket convention is preserved.
func PublicURL(cfg config.ObjectStorage, objectKey string) (string, bool) {
	key := strings.Trim(strings.TrimSpace(objectKey), "/")
	if key == "" {
		return "", false
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if base == "" {
		endpoint := strings.TrimSpace(cfg.Endpoint)
		bucket := strings.TrimSpace(cfg.Bucket)
		if endpoint == "" || bucket == "" {
			return "", false
		}
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
		endpoint = strings.Trim(endpoint, "/")
		if endpoint == "" {
			return "", false
		}
		if !strings.HasPrefix(strings.ToLower(endpoint), strings.ToLower(bucket)+".") {
			endpoint = bucket + "." + endpoint
		}
		base = scheme + "://" + endpoint
	}
	parts := strings.Split(key, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return base + "/" + strings.Join(parts, "/"), true
}
