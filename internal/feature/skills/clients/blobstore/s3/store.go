package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

var keyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type StoreClient struct {
	client *minio.Client
	bucket string
	prefix string
}

func New(o config.S3Options, transport http.RoundTripper) (*StoreClient, error) {
	creds := credentials.NewChainCredentials([]credentials.Provider{&credentials.EnvAWS{}, &credentials.IAM{}})
	if o.AccessKey != "" {
		creds = credentials.NewStaticV4(o.AccessKey, o.SecretKey, "")
	}
	lookup := minio.BucketLookupAuto
	if o.PathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(o.Endpoint, &minio.Options{Creds: creds, Secure: o.Secure, Transport: transport, Region: o.Region,
		BucketLookup: lookup})
	if err != nil {
		return nil, err
	}
	return &StoreClient{client: client, bucket: o.Bucket, prefix: o.Prefix}, nil
}

func (s *StoreClient) Name() string { return "s3" }

func (s *StoreClient) object(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", apperr.InternalDetail("invalid blob key " + key)
	}
	return s.prefix + key[:2] + "/" + key[2:], nil
}

func missing(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey"
}

func unavailable(err error) error {
	if err == nil {
		return nil
	}
	slog.Warn("skills storage request failed", "storage", "s3", "error", err)
	return skillstypes.StorageUnavailable
}

func (s *StoreClient) Put(ctx context.Context, key string, _ int64, r io.Reader) error {
	name, err := s.object(key)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return unavailable(err)
	}
	_, err = s.client.PutObject(ctx, s.bucket, name, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/octet-stream",
		UserMetadata: map[string]string{"sha256": key}, DisableMultipart: len(data) < 64<<20})
	return unavailable(err)
}

func (s *StoreClient) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	name, err := s.object(key)
	if err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, unavailable(err)
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		if missing(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, unavailable(err)
	}
	return obj, nil
}

func (s *StoreClient) Has(ctx context.Context, key string) (bool, error) {
	name, err := s.object(key)
	if err != nil {
		return false, err
	}
	_, err = s.client.StatObject(ctx, s.bucket, name, minio.StatObjectOptions{})
	if err != nil && missing(err) {
		return false, nil
	}
	return err == nil, unavailable(err)
}

func (s *StoreClient) Delete(ctx context.Context, key string) error {
	name, err := s.object(key)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.bucket, name, minio.RemoveObjectOptions{})
	if err != nil && !missing(err) && !errors.Is(err, context.Canceled) {
		return unavailable(err)
	}
	return nil
}
