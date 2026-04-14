package port

import (
	"context"
	"time"
)

type ObjectStorage interface {
	PresignUpload(ctx context.Context, key string, expiry time.Duration) (string, error)
	PresignView(ctx context.Context, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}
