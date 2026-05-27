package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrInvalidObjectKey = errors.New("object key is invalid")

type LocalStoreOptions struct {
	RootDir       string
	PublicBaseURL string
}

type LocalStore struct {
	rootDir       string
	publicBaseURL string
}

func NewLocalStore(options LocalStoreOptions) *LocalStore {
	publicBaseURL := strings.TrimRight(options.PublicBaseURL, "/")
	if publicBaseURL == "" {
		publicBaseURL = "/uploads"
	}
	return &LocalStore{
		rootDir:       options.RootDir,
		publicBaseURL: publicBaseURL,
	}
}

func (s *LocalStore) PutObject(ctx context.Context, input PutObjectInput) (PutObjectResult, error) {
	if err := validateObjectKey(input.Key); err != nil {
		return PutObjectResult{}, err
	}
	if input.Body == nil {
		return PutObjectResult{}, errors.New("object body is required")
	}
	if err := ctx.Err(); err != nil {
		return PutObjectResult{}, err
	}

	targetPath := filepath.Join(s.rootDir, filepath.FromSlash(input.Key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return PutObjectResult{}, err
	}
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return PutObjectResult{}, err
	}
	defer file.Close()

	written, err := io.Copy(file, input.Body)
	if err != nil {
		return PutObjectResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return PutObjectResult{}, err
	}

	return PutObjectResult{
		Key:         input.Key,
		URL:         s.publicBaseURL + "/" + path.Clean(input.Key),
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Size:        written,
	}, nil
}

func validateObjectKey(key string) error {
	cleaned := path.Clean(key)
	if key == "" || cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return ErrInvalidObjectKey
	}
	if path.IsAbs(cleaned) || filepath.IsAbs(key) || cleaned != key {
		return ErrInvalidObjectKey
	}
	return nil
}
