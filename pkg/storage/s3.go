package storage

import (
	"bytes"
	"context"
	"io"
	"time"

	"astro-scheduler/pkg/utils"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Storage struct {
	client *minio.Client
}

func NewS3Storage(cfg StorageConfig) (*S3Storage, error) {
	if cfg.Endpoint == "" {
		return nil, nil
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV2(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := client.MakeBucket(context.Background(), cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		}); err != nil {
			return nil, err
		}
		utils.Sugar.Infof("Created S3 bucket: %s", cfg.Bucket)
	}

	utils.Sugar.Infof("S3 storage initialized: %s/%s", cfg.Endpoint, cfg.Bucket)
	return &S3Storage{client: client}, nil
}

func (s *S3Storage) PutObject(ctx context.Context, bucket, key string, data io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := s.client.PutObject(ctx, bucket, key, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})

	if err != nil {
		utils.Sugar.Errorf("S3 put object failed: %s/%s - %v", bucket, key, err)
		return err
	}

	utils.Sugar.Debugf("S3 put object: %s/%s (%d bytes)", bucket, key, size)
	return nil
}

func (s *S3Storage) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, err
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         stat.Size,
		ETag:         stat.ETag,
		LastModified: stat.LastModified,
		ContentType:  stat.ContentType,
		Metadata:     stat.UserMetadata,
	}

	utils.Sugar.Debugf("S3 get object: %s/%s", bucket, key)
	return obj, info, nil
}

func (s *S3Storage) DeleteObject(ctx context.Context, bucket, key string) error {
	err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		utils.Sugar.Errorf("S3 delete object failed: %s/%s - %v", bucket, key, err)
		return err
	}

	utils.Sugar.Debugf("S3 delete object: %s/%s", bucket, key)
	return nil
}

func (s *S3Storage) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]*ObjectInfo, error) {
	var result []*ObjectInfo

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
		MaxKeys:   maxKeys,
	}

	for object := range s.client.ListObjects(ctx, bucket, opts) {
		if object.Err != nil {
			return nil, object.Err
		}

		result = append(result, &ObjectInfo{
			Key:          object.Key,
			Size:         object.Size,
			ETag:         object.ETag,
			LastModified: object.LastModified,
			ContentType:  object.ContentType,
		})

		if maxKeys > 0 && len(result) >= maxKeys {
			break
		}
	}

	return result, nil
}

func (s *S3Storage) HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	stat, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:          key,
		Size:         stat.Size,
		ETag:         stat.ETag,
		LastModified: stat.LastModified,
		ContentType:  stat.ContentType,
		Metadata:     stat.UserMetadata,
	}, nil
}

func (s *S3Storage) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	src := minio.CopySrcOptions{
		Bucket: bucket,
		Object: srcKey,
	}

	dst := minio.CopyDestOptions{
		Bucket: bucket,
		Object: dstKey,
	}

	_, err := s.client.CopyObject(ctx, dst, src)
	return err
}

func (s *S3Storage) PresignedGetURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, bucket, key, expires, nil)
	if err != nil {
		return "", err
	}

	return url.String(), nil
}

func (s *S3Storage) Close() error {
	s.client = nil
	return nil
}

func NewObjectStorage(cfg StorageConfig) (ObjectStorage, error) {
	switch cfg.Type {
	case "local":
		return NewLocalStorage(cfg.BasePath)
	case "s3", "minio", "oss", "cos":
		return NewS3Storage(cfg)
	default:
		return NewLocalStorage(cfg.BasePath)
	}
}

func (s *S3Storage) UploadBytes(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	return s.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), contentType)
}

func (s *S3Storage) DownloadBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	reader, _, err := s.GetObject(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
