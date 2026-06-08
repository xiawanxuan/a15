package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"astro-scheduler/pkg/utils"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if basePath == "" {
		basePath = "./data/storage"
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) PutObject(ctx context.Context, bucket, key string, data io.Reader, size int64, contentType string) error {
	fullPath := filepath.Join(s.basePath, bucket, key)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		return err
	}

	utils.Sugar.Debugf("Local storage: put object %s/%s (%d bytes)", bucket, key, size)
	return nil
}

func (s *LocalStorage) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	fullPath := filepath.Join(s.basePath, bucket, key)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
	}

	utils.Sugar.Debugf("Local storage: get object %s/%s", bucket, key)
	return file, info, nil
}

func (s *LocalStorage) DeleteObject(ctx context.Context, bucket, key string) error {
	fullPath := filepath.Join(s.basePath, bucket, key)

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	utils.Sugar.Debugf("Local storage: delete object %s/%s", bucket, key)
	return nil
}

func (s *LocalStorage) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]*ObjectInfo, error) {
	dirPath := filepath.Join(s.basePath, bucket)
	if prefix != "" {
		dirPath = filepath.Join(dirPath, prefix)
	}

	var result []*ObjectInfo

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(filepath.Join(s.basePath, bucket), path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		obj := &ObjectInfo{
			Key:          relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		}

		result = append(result, obj)

		if maxKeys > 0 && len(result) >= maxKeys {
			return io.EOF
		}

		return nil
	})

	if err != nil && err != io.EOF {
		return nil, err
	}

	return result, nil
}

func (s *LocalStorage) HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	fullPath := filepath.Join(s.basePath, bucket, key)

	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
	}, nil
}

func (s *LocalStorage) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	srcPath := filepath.Join(s.basePath, bucket, srcKey)
	dstPath := filepath.Join(s.basePath, bucket, dstKey)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (s *LocalStorage) PresignedGetURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	return "", nil
}

func (s *LocalStorage) Close() error {
	return nil
}
