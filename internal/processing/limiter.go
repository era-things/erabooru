package processing

import (
	"container/list"
	"context"
	"sync"
)

// Limiter is the shared concurrency budget for decode and inference work.
type Limiter chan struct{}

func NewLimiter(concurrency int) Limiter {
	return make(Limiter, concurrency)
}

type InferencePriority uint8

const (
	BackgroundInference InferencePriority = iota
	InteractiveInference
)

// InferenceLimiter bounds ONNX calls and always wakes interactive searches
// before queued background embedding work.
type InferenceLimiter struct {
	mu          sync.Mutex
	capacity    int
	active      int
	interactive list.List
	background  list.List
}

type inferenceWaiter struct {
	ready   chan struct{}
	queue   *list.List
	element *list.Element
	granted bool
}

func NewInferenceLimiter(concurrency int) *InferenceLimiter {
	return &InferenceLimiter{capacity: concurrency}
}

func (l *InferenceLimiter) Acquire(ctx context.Context, priority InferencePriority) (func(), error) {
	l.mu.Lock()
	if l.active < l.capacity {
		l.active++
		l.mu.Unlock()
		return l.releaseFunc(), nil
	}

	waiter := &inferenceWaiter{ready: make(chan struct{})}
	if priority == InteractiveInference {
		waiter.queue = &l.interactive
	} else {
		waiter.queue = &l.background
	}
	waiter.element = waiter.queue.PushBack(waiter)
	l.mu.Unlock()

	select {
	case <-waiter.ready:
		return l.releaseFunc(), nil
	case <-ctx.Done():
		l.mu.Lock()
		if !waiter.granted {
			waiter.queue.Remove(waiter.element)
			l.mu.Unlock()
			return nil, ctx.Err()
		}
		l.mu.Unlock()
		l.release()
		return nil, ctx.Err()
	}
}

func (l *InferenceLimiter) releaseFunc() func() {
	var once sync.Once
	return func() { once.Do(l.release) }
}

func (l *InferenceLimiter) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.active--

	queue := &l.interactive
	if queue.Len() == 0 {
		queue = &l.background
	}
	if element := queue.Front(); element != nil {
		waiter := element.Value.(*inferenceWaiter)
		queue.Remove(element)
		waiter.element = nil
		waiter.granted = true
		l.active++
		close(waiter.ready)
	}
}

func (l Limiter) Acquire(ctx context.Context) (func(), error) {
	select {
	case l <- struct{}{}:
		return func() { <-l }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
