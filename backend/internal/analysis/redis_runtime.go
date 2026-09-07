package analysis

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sagun-patwari/ai-career-platform/internal/workflowstate"
)

var workflowStateInitMu sync.Mutex

func (s *Service) ensureRedisState() error {
	if s.state != nil {
		return nil
	}

	workflowStateInitMu.Lock()
	defer workflowStateInitMu.Unlock()

	if s.state != nil {
		return nil
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	ttlMinutes := 60
	if raw := os.Getenv("WORKFLOW_STATE_TTL_MINUTES"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid WORKFLOW_STATE_TTL_MINUTES: %q", raw)
		}
		ttlMinutes = n
	}

	store, err := workflowstate.New(redisURL, time.Duration(ttlMinutes)*time.Minute)
	if err != nil {
		return err
	}
	s.state = NewRedisStateAdapter(store)
	return nil
}
