package llm

import (
	"context"
	"errors"
	"strings"
	"time"
)

type GuardrailPolicy struct {
	MaxRetries     int
	RequestTimeout time.Duration
	MaxInputChars  int
	MaxOutputChars int
}

func DefaultGuardrailPolicy() GuardrailPolicy {
	return GuardrailPolicy{MaxRetries: 1, RequestTimeout: 35 * time.Second, MaxInputChars: 18000, MaxOutputChars: 12000}
}

func ClampContext(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	head := max * 2 / 3
	tail := max - head
	return s[:head] + "\n...[context compressed]...\n" + s[len(s)-tail:]
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	x := strings.ToLower(err.Error())
	for _, bad := range []string{"400", "401", "403", "429", "invalid argument", "permission", "quota", "resource_exhausted"} {
		if strings.Contains(x, bad) {
			return false
		}
	}
	return true
}
