package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// workflowAgent is one independently executable step in the analysis graph.
type workflowAgent struct {
	name      string
	dependsOn []string
	run       func(context.Context) error
}

// runWorkflow executes every dependency-ready group concurrently. For this
// small, fixed DAG the level-by-level scheduler stays simple and easy to audit.
func runWorkflow(ctx context.Context, agents []workflowAgent) error {
	done := make(map[string]bool, len(agents))
	for len(done) < len(agents) {
		if err := ctx.Err(); err != nil {
			return err
		}

		ready := make([]workflowAgent, 0, len(agents))
		for _, a := range agents {
			if done[a.name] {
				continue
			}
			ok := true
			for _, dep := range a.dependsOn {
				if !done[dep] {
					ok = false
					break
				}
			}
			if ok {
				ready = append(ready, a)
			}
		}
		if len(ready) == 0 {
			return errors.New("agent dependency cycle or missing dependency")
		}

		var wg sync.WaitGroup
		errCh := make(chan error, len(ready))
		for _, a := range ready {
			wg.Add(1)
			go func(a workflowAgent) {
				defer wg.Done()

				start := time.Now()
				slog.Info("agent started", "agent", a.name)

				if err := a.run(ctx); err != nil {
					slog.Error(
						"agent failed",
						"agent", a.name,
						"duration", time.Since(start),
						"error", err,
					)
					errCh <- fmt.Errorf("%s agent: %w", a.name, err)
					return
				}

				slog.Info(
					"agent finished",
					"agent", a.name,
					"duration", time.Since(start),
				)
			}(a)

		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			return err
		}
		for _, a := range ready {
			done[a.name] = true
		}
	}
	return nil
}
