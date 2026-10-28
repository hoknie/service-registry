package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

func testStore(t *testing.T) *StoreClient {
	t.Helper()
	raw := os.Getenv("TEST_S3_URL")
	if raw == "" {
		t.Skip("TEST_S3_URL is not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := u.User.Password()
	o := config.S3Options{Endpoint: u.Host, Secure: u.Scheme == "https", Region: "us-east-1",
		Bucket: "svcr-tests", Prefix: uuid.Must(uuid.NewV7()).String() + "/",
		AccessKey: u.User.Username(), SecretKey: secret, PathStyle: true}
	s, err := New(o, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		err = s.client.MakeBucket(context.Background(), o.Bucket, minio.MakeBucketOptions{Region: o.Region})
		if code := minio.ToErrorResponse(err).Code; code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists" {
			err = nil
		}
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestS3StoreKeepsObjectsByKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := testStore(t)
	sum := sha256.Sum256([]byte("hello"))
	key := hex.EncodeToString(sum[:])
	if has, err := s.Has(ctx, key); err != nil || has {
		t.Fatal(has, err)
	}
	if err := s.Put(ctx, key, 5, strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	if has, err := s.Has(ctx, key); err != nil || !has {
		t.Fatal(has, err)
	}
	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "hello" {
		t.Fatal(string(data))
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, key); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestS3StoreReportsAnUnreachableBucket(t *testing.T) {
	t.Parallel()
	s, err := New(config.S3Options{Endpoint: "127.0.0.1:1", Region: "us-east-1", Bucket: "skills", AccessKey: "a", SecretKey: "b",
		PathStyle: true}, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.Has(ctx, strings.Repeat("a", 64)); apperr.From(err).Code != "skills.storage_unavailable" {
		t.Fatal(err)
	}
}
