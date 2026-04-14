package port

import "context"

type MessageQueue interface {
	Publish(ctx context.Context, subject string, data []byte) error
}
