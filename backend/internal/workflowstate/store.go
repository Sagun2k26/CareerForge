package workflowstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrConflict = errors.New("workflow state version conflict")

type Envelope struct {
	Version int64           `json:"version"`
	Data    json.RawMessage `json:"data"`
}

type Store struct {
	redis *redis.Client
	ttl   time.Duration
}

func New(redisURL string, ttl time.Duration) (*Store, error) {
	if redisURL == "" {
		return nil, errors.New("REDIS_URL is required for distributed workflow state")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis workflow state: %w", err)
	}
	return &Store{redis: c, ttl: ttl}, nil
}

func (s *Store) Close() error         { return s.redis.Close() }
func (s *Store) key(id string) string { return "careerforge:workflow:" + id }

func (s *Store) Init(ctx context.Context, id string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	env, _ := json.Marshal(Envelope{Version: 1, Data: b})
	_, err = s.redis.SetNX(ctx, s.key(id), env, s.ttl).Result()
	return err
}

func (s *Store) Load(ctx context.Context, id string, dst any) (int64, error) {
	raw, err := s.redis.Get(ctx, s.key(id)).Bytes()
	if err != nil {
		return 0, err
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, err
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		return 0, err
	}
	return env.Version, nil
}

func (s *Store) Update(ctx context.Context, id string, fn func(json.RawMessage) (any, error)) error {
	key := s.key(id)
	for attempt := 0; attempt < 5; attempt++ {
		err := s.redis.Watch(ctx, func(tx *redis.Tx) error {
			raw, err := tx.Get(ctx, key).Bytes()
			if err != nil {
				return err
			}
			var env Envelope
			if err := json.Unmarshal(raw, &env); err != nil {
				return err
			}
			next, err := fn(env.Data)
			if err != nil {
				return err
			}
			b, err := json.Marshal(next)
			if err != nil {
				return err
			}
			out, _ := json.Marshal(Envelope{Version: env.Version + 1, Data: b})
			_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
				p.Set(ctx, key, out, s.ttl)
				return nil
			})
			return err
		}, key)
		if err == nil {
			return nil
		}
		if err == redis.TxFailedErr {
			continue
		}
		return err
	}
	return ErrConflict
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.redis.Del(ctx, s.key(id)).Err()
}
