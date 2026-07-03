// Package worker is a tiny in-process job queue backed by a buffered channel and
// a pool of goroutines. Slow work (resume parsing, embedding generation) is
// enqueued so HTTP handlers return immediately and the API stays responsive.
package worker

import (
	"context"
	"log/slog"
	"sync"
)

// Job is a unit of background work.
type Job struct {
	Name string
	Run  func(ctx context.Context) error
}

// Pool runs jobs concurrently.
type Pool struct {
	jobs   chan Job
	wg     sync.WaitGroup
	logger *slog.Logger
}

// NewPool starts `workers` goroutines listening on a queue of size `buffer`.
func NewPool(ctx context.Context, workers, buffer int, logger *slog.Logger) *Pool {
	p := &Pool{jobs: make(chan Job, buffer), logger: logger}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.loop(ctx)
	}
	return p
}

func (p *Pool) loop(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			// Detach from request lifecycles but respect shutdown.
			if err := job.Run(ctx); err != nil {
				p.logger.Error("background job failed", "job", job.Name, "error", err)
			} else {
				p.logger.Info("background job done", "job", job.Name)
			}
		}
	}
}

// Enqueue submits a job. It never blocks the caller: if the queue is full the
// job runs inline in a new goroutine so work is never dropped.
func (p *Pool) Enqueue(job Job) {
	select {
	case p.jobs <- job:
	default:
		go func() {
			if err := job.Run(context.Background()); err != nil {
				p.logger.Error("inline job failed", "job", job.Name, "error", err)
			}
		}()
	}
}

// Shutdown stops accepting work and waits for in-flight jobs.
func (p *Pool) Shutdown() {
	close(p.jobs)
	p.wg.Wait()
}
