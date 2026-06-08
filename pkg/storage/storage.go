package storage

import (
	"context"
	"io"
	"time"
)

type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
	ContentType  string
	Metadata     map[string]string
}

type ObjectStorage interface {
	PutObject(ctx context.Context, bucket, key string, data io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]*ObjectInfo, error)
	HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)
	CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error
	PresignedGetURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)
	Close() error
}

type StorageConfig struct {
	Type        string
	Endpoint    string
	AccessKey   string
	SecretKey   string
	Bucket      string
	Region      string
	UseSSL      bool
	BasePath    string
}
