package analysis

import (
	"context"
	"encoding/json"
	"github.com/sagun-patwari/ai-career-platform/internal/workflowstate"
)

type RedisStateAdapter struct{ store *workflowstate.Store }

func NewRedisStateAdapter(s *workflowstate.Store) *RedisStateAdapter {
	return &RedisStateAdapter{store: s}
}
func (a *RedisStateAdapter) Init(ctx context.Context, id string, state any) error {
	return a.store.Init(ctx, id, state)
}
func (a *RedisStateAdapter) Load(ctx context.Context, id string, dst any) (int64, error) {
	return a.store.Load(ctx, id, dst)
}
func (a *RedisStateAdapter) Patch(ctx context.Context, id string, fn func(json.RawMessage) (any, error)) error {
	return a.store.Update(ctx, id, fn)
}
func (a *RedisStateAdapter) Delete(ctx context.Context, id string) error {
	return a.store.Delete(ctx, id)
}
