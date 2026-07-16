package processing

import "context"

// Limiter is the shared concurrency budget for decode and inference work.
type Limiter chan struct{}

func NewLimiter(concurrency int) Limiter {
	return make(Limiter, concurrency)
}

func (l Limiter) Acquire(ctx context.Context) (func(), error) {
	select {
	case l <- struct{}{}:
		return func() { <-l }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
