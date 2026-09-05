package analysis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRunWorkflowRespectsDependenciesAndRunsReadyAgentsTogether(t *testing.T) {
	var mu sync.Mutex
	done := map[string]bool{}
	started := make(chan string, 2)
	release := make(chan struct{})

	agents := []workflowAgent{
		{name: "resume", run: func(context.Context) error {
			mu.Lock()
			done["resume"] = true
			mu.Unlock()
			return nil
		}},
		{name: "role_fit", dependsOn: []string{"resume"}, run: func(context.Context) error {
			started <- "role_fit"
			<-release
			mu.Lock()
			done["role_fit"] = true
			mu.Unlock()
			return nil
		}},
		{name: "skill_gap", dependsOn: []string{"resume"}, run: func(context.Context) error {
			started <- "skill_gap"
			<-release
			mu.Lock()
			done["skill_gap"] = true
			mu.Unlock()
			return nil
		}},
		{name: "final", dependsOn: []string{"role_fit", "skill_gap"}, run: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			if !done["role_fit"] || !done["skill_gap"] {
				return errors.New("final agent ran before its dependencies")
			}
			done["final"] = true
			return nil
		}},
	}

	errCh := make(chan error, 1)
	go func() { errCh <- runWorkflow(context.Background(), agents) }()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatal("dependency-ready agents did not start concurrently")
		}
	}
	close(release)

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if !done["final"] {
		t.Fatal("final agent did not run")
	}
}

func TestRunWorkflowDetectsCycle(t *testing.T) {
	err := runWorkflow(context.Background(), []workflowAgent{
		{name: "a", dependsOn: []string{"b"}, run: func(context.Context) error { return nil }},
		{name: "b", dependsOn: []string{"a"}, run: func(context.Context) error { return nil }},
	})
	if err == nil {
		t.Fatal("expected dependency cycle error")
	}
}
