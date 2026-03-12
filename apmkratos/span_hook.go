package apmkratos

import (
	"context"

	"go.elastic.co/apm/v2"
)

// ApmSpanHook tracks an Elastic APM span lifecycle via Start and Close
// Implements Start(ctx, spanName) and Close() matching authkratos.SpanHook interface implicitly
//
// ApmSpanHook 通过 Start 和 Close 管理 Elastic APM span 的生命周期
// 隐式实现 authkratos.SpanHook 接口的 Start(ctx, spanName) 和 Close() 方法
type ApmSpanHook struct {
	span *apm.Span
}

// Start creates an APM span from current transaction context
//
// Start 从当前事务上下文中创建 APM span
func (h *ApmSpanHook) Start(ctx context.Context, spanName string) {
	h.span, _ = apm.StartSpan(ctx, spanName, "custom")
}

// Close ends the APM span if it was created
//
// Close 结束 APM span（如果已创建）
func (h *ApmSpanHook) Close() {
	if h.span != nil {
		h.span.End()
	}
}

// NewSpanHook creates a fresh ApmSpanHook instance
// The returned value implicitly satisfies authkratos.SpanHook interface
// Usage:
//
//	cfg.WithNewSpanHook(func() authkratos.SpanHook { return apmkratos.NewSpanHook() })
//
// NewSpanHook 创建一个新的 ApmSpanHook 实例
// 返回值隐式满足 authkratos.SpanHook 接口
func NewSpanHook() *ApmSpanHook {
	return &ApmSpanHook{}
}
