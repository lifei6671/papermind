package storage

import (
	"context"
	"io"
)

type PutObjectInput struct {
	Key         string
	FileName    string
	ContentType string
	Size        int64
	Body        io.Reader
}

type PutObjectResult struct {
	Key         string
	URL         string
	FileName    string
	ContentType string
	Size        int64
}

// ObjectStore 是通用对象存储边界；S3、OSS、MinIO 等远端存储只需要实现该接口。
type ObjectStore interface {
	PutObject(ctx context.Context, input PutObjectInput) (PutObjectResult, error)
}
