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

// InferenceLimiter gives interactive searches priority over background work.
// Background work pauses while searches are queued or active. When capacity is
// greater than one, one slot is reserved for an arriving search. Running ONNX
// calls cannot be preempted; callers must release between bounded inference batches.
type InferenceLimiter struct {
	mu                sync.Mutex
	capacity          int
	activeInteractive int
	activeBackground  int
	interactive       list.List
	background        list.List
}

type inferenceWaiter struct {
	ready   chan struct{}
	queue   *list.List
	element *list.Element
	granted bool
}

func NewInferenceLimiter(concurrency int) *InferenceLimiter {
	return &InferenceLimiter{capacity: max(1, concurrency)}
}

func (l *InferenceLimiter) Acquire(ctx context.Context, priority InferencePriority) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	waiter := &inferenceWaiter{ready: make(chan struct{})}
	if priority == InteractiveInference {
		waiter.queue = &l.interactive
	} else {
		waiter.queue = &l.background
	}
	waiter.element = waiter.queue.PushBack(waiter)
	l.dispatchLocked()
	l.mu.Unlock()

	select {
	case <-waiter.ready:
		if err := ctx.Err(); err != nil {
			l.release(priority)
			return nil, err
		}
		return l.releaseFunc(priority), nil
	case <-ctx.Done():
		l.mu.Lock()
		if !waiter.granted {
			waiter.queue.Remove(waiter.element)
			l.dispatchLocked()
			l.mu.Unlock()
			return nil, ctx.Err()
		}
		l.mu.Unlock()
		l.release(priority)
		return nil, ctx.Err()
	}
}

func (l *InferenceLimiter) releaseFunc(priority InferencePriority) func() {
	var once sync.Once
	return func() { once.Do(func() { l.release(priority) }) }
}

func (l *InferenceLimiter) release(priority InferencePriority) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if priority == InteractiveInference {
		l.activeInteractive--
	} else {
		l.activeBackground--
	}
	l.dispatchLocked()
}

func (l *InferenceLimiter) dispatchLocked() {
	for l.activeInteractive+l.activeBackground < l.capacity && l.interactive.Len() > 0 {
		l.grantLocked(&l.interactive)
		l.activeInteractive++
	}
	if l.activeInteractive > 0 || l.interactive.Len() > 0 {
		return
	}
	backgroundCapacity := max(1, l.capacity-1)
	for l.activeBackground < backgroundCapacity && l.background.Len() > 0 {
		l.grantLocked(&l.background)
		l.activeBackground++
	}
}

func (l *InferenceLimiter) grantLocked(queue *list.List) {
	element := queue.Front()
	waiter := element.Value.(*inferenceWaiter)
	queue.Remove(element)
	waiter.element = nil
	waiter.granted = true
	close(waiter.ready)
}

func (l Limiter) Acquire(ctx context.Context) (func(), error) {
	select {
	case l <- struct{}{}:
		return func() { <-l }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
