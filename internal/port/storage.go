package port

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	PresignUpload(ctx context.Context, key string, expiry time.Duration) (string, error)
	PresignView(ctx context.Context, key string, fileName string, expiry time.Duration) (string, error)
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}
