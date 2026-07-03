package worker

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestPoolRunsEnqueuedJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool := NewPool(ctx, 2, 8, logger)

	var mu sync.Mutex
	count := 0
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.Enqueue(Job{Name: "test", Run: func(context.Context) error {
			defer wg.Done()
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		}})
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("jobs did not complete in time")
	}

	mu.Lock()
	defer mu.Unlock()
	if count != 5 {
		t.Fatalf("expected 5 jobs to run, got %d", count)
	}
}
