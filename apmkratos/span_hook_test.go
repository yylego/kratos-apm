package apmkratos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSpanHook(t *testing.T) {
	hook := NewSpanHook()
	require.NotNil(t, hook)

	// No active APM transaction, span is nil — should not panic
	// 无活跃 APM 事务，span 为 nil — 不应 panic
	hook.Start(context.Background(), "test-span")
	hook.Close()
}
