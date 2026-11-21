// Package apmkratos connects Kratos services to Elastic APM
// It gives a middleware that starts one APM transaction on each gRPC and HTTP request,
// passes the W3C TraceContext on to outgoing gRPC requests, and reports errors and panics to APM
//
// apmkratos 把 Kratos 服务接到 Elastic APM
// 提供的中间件为每个 gRPC、HTTP 请求开一个 APM 事务，把 W3C TraceContext 传给下游的 gRPC 请求，并把错误和 panic 上报到 APM
package apmkratos

import (
	"context"
	stderrors "errors"
	nethttp "net/http"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/yylego/elasticapm"
	"go.elastic.co/apm/module/apmhttp/v2"
	"go.elastic.co/apm/v2"
)

// Middleware gives the trace middleware and the panic middleware as one
// The trace part wraps the panic part, so a panic is reported inside the transaction of its request
//
// Middleware 把追踪中间件和 panic 中间件合成一个
// 追踪在外、panic 捕获在内，panic 上报时就在所属请求的事务里
func Middleware() middleware.Middleware {
	var m1 = NewApmTraceMiddleware() // Starts the transaction // 开启事务
	var m2 = NewRecoveryMiddleware() // Reports panics // 上报 panic
	return func(handleFunc middleware.Handler) middleware.Handler {
		return m1(m2(handleFunc))
	}
}

// NewApmTraceMiddleware creates the trace middleware: it starts an APM transaction on each gRPC and HTTP request
// The incoming W3C traceparent and tracestate continue the upstream trace, and both go into the outgoing gRPC metadata
// An HTTP transaction is named with the method and the route template, such as "GET /articles/{id}";
// a gRPC transaction is named with the Kratos operation
// When the handle func returns an error, the error is sent to APM with the transaction as a handled one
// A request with no server transport in the context passes through untouched
//
// NewApmTraceMiddleware 创建追踪中间件：为每个 gRPC、HTTP 请求开一个 APM 事务
// 读取请求带来的 W3C traceparent 和 tracestate 接上上游的链路，再把两者写进发往下游的 gRPC metadata
// HTTP 事务用请求方法加路由模板命名，如 "GET /articles/{id}"；gRPC 事务用 Kratos 的 operation 命名
// 处理函数返回错误时，把错误作为已处理的错误、连同事务一起上报到 APM
// context 里没有服务端传输信息的请求原样放行
func NewApmTraceMiddleware() middleware.Middleware {
	return func(handleFunc middleware.Handler) middleware.Handler {
		return func(ctx context.Context, arg any) (any, error) {
			tsp, ok := transport.FromServerContext(ctx)
			if !ok {
				return handleFunc(ctx, arg)
			}

			switch tsp.Kind() {
			case transport.KindGRPC:
				var opts apm.TransactionOptions
				opts.TraceContext = parseIncomingMetadataTraceContext(tsp.RequestHeader())
				apmTx := apm.DefaultTracer().StartTransactionOptions(tsp.Operation(), "request", opts)
				ctx = elasticapm.ContextWithTraceGrpcOutgoing(ctx, apmTx)
				// Deferred funcs run in reverse: the context is set first, then the transaction ends
				// defer 倒序执行：先设置上下文，再结束事务
				defer apmTx.End()
				defer setGRPCContext(&apmTx.Context)
			case transport.KindHTTP:
				httpTransport := HTTPTransportFromContext(ctx)
				req := httpTransport.Request()
				requestName := resolveHTTPRequestName(httpTransport)
				apmTx, body, req := apmhttp.StartTransactionWithBody(apm.DefaultTracer(), requestName, req)
				ctx = elasticapm.ContextWithTraceGrpcOutgoing(ctx, apmTx)
				defer apmTx.End()
				defer setHTTPContext(&apmTx.Context, body, req)
			}
			res, erk := handleFunc(ctx, arg)
			// An error that comes out of a panic is skipped: NewRecoveryApmOptions has sent the panic to APM
			// 由 panic 产生的错误跳过不报：NewRecoveryApmOptions 已经把这次 panic 报给 APM
			if erk != nil && !stderrors.Is(erk, errPanicReported) {
				// The transaction stays open here: ending it is the job of the defer above,
				// which runs once the context is set
				// 这里不结束事务：结束由上面的 defer 负责，它在上下文设置好之后才执行
				if apmTx := apm.TransactionFromContext(ctx); apmTx != nil {
					e := apm.DefaultTracer().NewError(erk)
					e.Handled = true
					e.SetTransaction(apmTx)
					e.Send()
				}
			}
			return res, erk
		}
	}
}

// resolveHTTPRequestName names the HTTP transaction with the method and the route template,
// so "/articles/123" and "/articles/456" count as one transaction: "GET /articles/{id}"
// With no route template it takes the name that apmhttp gives: the method and the request path
//
// resolveHTTPRequestName 用请求方法加路由模板给 HTTP 事务命名，
// "/articles/123" 和 "/articles/456" 就归成同一个事务："GET /articles/{id}"
// 拿不到路由模板时用 apmhttp 给的名字：请求方法加请求路径
func resolveHTTPRequestName(httpTransport *http.Transport) string {
	req := httpTransport.Request()
	pathTemplate := httpTransport.PathTemplate()
	if pathTemplate == "" {
		return apmhttp.ServerRequestName(req)
	}
	return req.Method + " " + pathTemplate
}

// setGRPCContext marks the transaction as coming from the Kratos framework
//
// setGRPCContext 把事务标成来自 Kratos 框架
func setGRPCContext(ctx *apm.Context) {
	ctx.SetFramework("kratos", "v3")
	// More gRPC context can be set here when needed
	// 需要时可以在这里设置更多 gRPC 上下文
	// Reference: https://github.com/elastic/apm-agent-go/blob/main/module/apmgrpc/server.go
}

// setHTTPContext marks the transaction as coming from the Kratos framework, and attaches the HTTP request and its captured body
// The response status and headers are not set: Kratos writes the response once the middleware has returned
//
// setHTTPContext 把事务标成来自 Kratos 框架，并附上 HTTP 请求和捕获到的请求体
// 响应状态码和响应头没有设置：Kratos 在中间件返回之后才写响应
func setHTTPContext(ctx *apm.Context, body *apm.BodyCapturer, req *nethttp.Request) {
	ctx.SetFramework("kratos", "v3")
	ctx.SetHTTPRequest(req)
	ctx.SetHTTPRequestBody(body)
}

// parseIncomingMetadataTraceContext reads the W3C traceparent and tracestate from the request headers
// It returns a blank TraceContext when traceparent is absent, and also when it does not parse
//
// parseIncomingMetadataTraceContext 从请求头读取 W3C 的 traceparent 和 tracestate
// 没有 traceparent、或解析不出来时，返回空的 TraceContext
func parseIncomingMetadataTraceContext(md transport.Header) apm.TraceContext {
	if value := md.Get(strings.ToLower(apmhttp.W3CTraceparentHeader)); value != "" {
		apmTraceContext, err := apmhttp.ParseTraceparentHeader(value)
		if err != nil {
			return apm.TraceContext{}
		}
		apmTraceContext.State, _ = apmhttp.ParseTracestateHeader(md.Get(strings.ToLower(apmhttp.TracestateHeader)))
		return apmTraceContext
	}
	return apm.TraceContext{}
}

// HTTPTransportFromContext returns the Kratos HTTP transport of the request in the context
// It returns nil when the request is not an HTTP one
//
// HTTPTransportFromContext 返回 context 里这次请求的 Kratos HTTP 传输对象
// 请求不是 HTTP 请求时返回 nil
func HTTPTransportFromContext(ctx context.Context) *http.Transport {
	if tsp, ok := transport.FromServerContext(ctx); ok {
		if tsp.Kind() == transport.KindHTTP {
			if res, ok := tsp.(*http.Transport); ok {
				return res
			}
		}
	}
	return nil
}

// NewRecoveryMiddleware creates the panic middleware: it recovers a panic, reports it to APM, and responds 500 INTERNAL_SERVER_ERROR
// Put it inside the trace middleware (Middleware does this), so the panic lands in the transaction of its request
//
// NewRecoveryMiddleware 创建 panic 中间件：捕获 panic、上报到 APM，并返回 500 INTERNAL_SERVER_ERROR
// 要放在追踪中间件里面（Middleware 就是这样组合的），panic 才会记到所属请求的事务里
func NewRecoveryMiddleware() middleware.Middleware {
	return recovery.Recovery(NewRecoveryApmOptions(func(format string, args ...any) *errors.Error {
		return errors.Newf(500, "INTERNAL_SERVER_ERROR", format, args...)
	}))
}

// errPanicReported is the cause of each error that NewRecoveryApmOptions returns once the panic is sent to APM
// The trace middleware sees it and does not send that error again, so one panic is one error in APM
//
// errPanicReported 是 NewRecoveryApmOptions 把 panic 报给 APM 之后，返回的错误所带的 cause
// 追踪中间件认出它就不再上报这个错误，一次 panic 在 APM 里只有一条错误
var errPanicReported = stderrors.New("panic reported to apm")

// NewRecoveryApmOptions gives the Kratos recovery option that reports a panic to APM, with its stack trace
// efn builds the error to respond with, it gets a format and the recovered value
// The error from efn goes back with errPanicReported as its cause, which marks the panic as reported
// With no APM transaction in the context the panic is not reported, and the message says so
//
// NewRecoveryApmOptions 返回 Kratos recovery 的选项：发生 panic 时连同调用栈上报到 APM
// efn 用来构造要返回的错误，收到的是格式串和 recover 到的值
// efn 给的错误返回时 cause 设为 errPanicReported，标明这次 panic 已经上报
// context 里没有 APM 事务时不上报，返回的消息里会注明
func NewRecoveryApmOptions(efn func(format string, args ...any) *errors.Error) recovery.Option {
	return recovery.WithHandler(func(ctx context.Context, req, erx any) error {
		apmTx := apm.TransactionFromContext(ctx)
		if apmTx == nil {
			// No transaction: the trace middleware has to wrap this one
			// 没有事务：追踪中间件要包在它外面
			return efn("service panic: %v (not reported, no apm transaction)", erx)
		}
		// The transaction stays open: the middleware that started it ends it
		// 这里不结束事务：事务由开启它的中间件结束
		e := apm.DefaultTracer().Recovered(erx)
		e.SetTransaction(apmTx)
		e.Send()
		return efn("service panic: %v", erx).WithCause(errPanicReported)
	})
}
