package apmkratos

import (
	"context"
	"encoding/hex"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/apmtest"
	"go.elastic.co/apm/v2/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

// The trace context an upstream service sends: trace id, parent span id, and tracestate
// 上游服务发来的链路信息：trace id、上一级的 span id、tracestate
const (
	upstreamTraceID     = "0af7651916cd43dd8448eb211c80319c"
	upstreamSpanID      = "b7ad6b7169203331"
	upstreamTraceparent = "00-" + upstreamTraceID + "-" + upstreamSpanID + "-01"
	upstreamTracestate  = "rojo=00f067aa0ba902b7"
)

// startRecording makes the default APM agent keep its events inside the process, so the test needs no APM service
// startRecording 让默认的 APM agent 把事件留在内存里，测试不需要 APM 服务在线
func startRecording(t *testing.T) *apmtest.RecordingTracer {
	recording := apmtest.NewRecordingTracer()
	apm.SetDefaultTracer(recording.Tracer)
	t.Cleanup(func() {
		apm.SetDefaultTracer(nil)
	})
	return recording
}

// listArticles replies with the trace headers the middleware put into the outgoing gRPC metadata,
// and with the request path taken from the HTTP transport
//
// listArticles 返回中间件写进下游 gRPC metadata 的链路请求头，以及从 HTTP 传输对象取到的请求路径
func listArticles(ctx context.Context, req any) (any, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	return map[string]string{
		"path":        HTTPTransportFromContext(ctx).Request().URL.Path,
		"traceparent": strings.Join(md.Get("traceparent"), ","),
		"tracestate":  strings.Join(md.Get("tracestate"), ","),
	}, nil
}

// listComments replies with the request path, its route has the article id as a path variable
// listComments 返回请求路径，它的路由把文章 id 作为路径变量
func listComments(ctx context.Context, req any) (any, error) {
	return map[string]string{"path": HTTPTransportFromContext(ctx).Request().URL.Path}, nil
}

// findArticle fails with a not-found Kratos error
// findArticle 返回一个 Kratos 的 not-found 错误
func findArticle(ctx context.Context, req any) (any, error) {
	return nil, errors.NotFound("ARTICLE_NOT_FOUND", "article not found")
}

// firstArticle panics: it takes the first item of a list that has no items
// firstArticle 会 panic：它从空列表里取第一项
func firstArticle(ctx context.Context, req any) (any, error) {
	var titles []string
	return titles[0], nil
}

// newTestServer starts a Kratos HTTP server with Middleware and a set of routes
// newTestServer 启动挂着 Middleware 的 Kratos HTTP 服务，带一组路由
func newTestServer(t *testing.T) *httptest.Server {
	srv := http.NewServer(http.Middleware(Middleware()))
	route := srv.Route("/")
	for path, handleFunc := range map[string]middleware.Handler{
		"/articles":               listArticles,
		"/articles/find":          findArticle,
		"/articles/first":         firstArticle,
		"/articles/{id}/comments": listComments,
	} {
		route.GET(path, func(ctx http.Context) error {
			res, err := ctx.Middleware(handleFunc)(ctx, nil)
			if err != nil {
				return err
			}
			return ctx.Result(200, res)
		})
	}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	return server
}

// sendRequest sends a GET request with the given headers and returns the status code and the body
// sendRequest 发一个带指定请求头的 GET 请求，返回状态码和响应体
func sendRequest(t *testing.T, address string, headers map[string]string) (int, string) {
	request, err := nethttp.NewRequest(nethttp.MethodGet, address, nil)
	require.NoError(t, err)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := nethttp.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, response.Body.Close())
	}()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(body)
}

// requireKratosFramework checks the transaction is marked as coming from Kratos v3
// requireKratosFramework 检查事务标成了来自 Kratos v3
func requireKratosFramework(t *testing.T, transaction model.Transaction) {
	require.NotNil(t, transaction.Context)
	require.NotNil(t, transaction.Context.Service)
	require.Equal(t, &model.Framework{Name: "kratos", Version: "v3"}, transaction.Context.Service.Framework)
}

func TestMiddleware(t *testing.T) {
	recording := startRecording(t)
	server := newTestServer(t)

	code, body := sendRequest(t, server.URL+"/articles", nil)
	require.Equal(t, 200, code)

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Empty(t, payloads.Errors)

	transaction := payloads.Transactions[0]
	require.Equal(t, "GET /articles", transaction.Name)
	require.Equal(t, "request", transaction.Type)
	require.Equal(t, "success", transaction.Outcome)
	requireKratosFramework(t, transaction)
	require.Equal(t, "GET", transaction.Context.Request.Method)

	// The outgoing traceparent names this transaction as the parent of the downstream one
	// A trace that starts here gets its tracestate from the agent: es=s:1 is the Elastic item with the sample rate 1
	// 发往下游的 traceparent 把这次事务作为下游事务的上一级
	// 从这里起头的链路，tracestate 由 agent 生成：es=s:1 是 Elastic 的条目，采样率为 1
	traceparent := "00-" + hex.EncodeToString(transaction.TraceID[:]) + "-" + hex.EncodeToString(transaction.ID[:]) + "-01"
	require.Equal(t, `{"path":"/articles","traceparent":"`+traceparent+`","tracestate":"es=s:1"}`, body)
}

// TestMiddleware_UpstreamTrace checks a request with traceparent and tracestate continues the upstream trace,
// and both go on into the outgoing gRPC metadata
//
// TestMiddleware_UpstreamTrace 检查带 traceparent 和 tracestate 的请求接上上游的链路，两者接着写进下游的 gRPC metadata
func TestMiddleware_UpstreamTrace(t *testing.T) {
	recording := startRecording(t)
	server := newTestServer(t)

	code, body := sendRequest(t, server.URL+"/articles", map[string]string{
		"Traceparent": upstreamTraceparent,
		"Tracestate":  upstreamTracestate,
	})
	require.Equal(t, 200, code)

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 1)

	transaction := payloads.Transactions[0]
	require.Equal(t, upstreamTraceID, hex.EncodeToString(transaction.TraceID[:]))
	require.Equal(t, upstreamSpanID, hex.EncodeToString(transaction.ParentID[:]))

	traceparent := "00-" + upstreamTraceID + "-" + hex.EncodeToString(transaction.ID[:]) + "-01"
	require.Equal(t, `{"path":"/articles","traceparent":"`+traceparent+`","tracestate":"`+upstreamTracestate+`"}`, body)
}

// TestMiddleware_RouteTemplate checks requests with different values in a path variable count as one transaction,
// named with the method and the route template
//
// TestMiddleware_RouteTemplate 检查只有路径变量不同的请求归成同一个事务，事务用请求方法加路由模板命名
func TestMiddleware_RouteTemplate(t *testing.T) {
	recording := startRecording(t)
	server := newTestServer(t)

	code, body := sendRequest(t, server.URL+"/articles/123/comments", nil)
	require.Equal(t, 200, code)
	require.Equal(t, `{"path":"/articles/123/comments"}`, body)

	code, body = sendRequest(t, server.URL+"/articles/456/comments", nil)
	require.Equal(t, 200, code)
	require.Equal(t, `{"path":"/articles/456/comments"}`, body)

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 2)
	require.Equal(t, "GET /articles/{id}/comments", payloads.Transactions[0].Name)
	require.Equal(t, "GET /articles/{id}/comments", payloads.Transactions[1].Name)
}

// TestMiddleware_HandleError checks the error of the handle func goes back unchanged,
// and reaches APM bound to a transaction that has its request context
//
// TestMiddleware_HandleError 检查处理函数的错误原样返回，并且上报到 APM 时挂在事务上，事务的请求上下文也在
func TestMiddleware_HandleError(t *testing.T) {
	recording := startRecording(t)
	server := newTestServer(t)

	code, body := sendRequest(t, server.URL+"/articles/find", nil)
	require.Equal(t, 404, code)
	require.Equal(t, `{"code":404,"reason":"ARTICLE_NOT_FOUND","message":"article not found"}`, body)

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Len(t, payloads.Errors, 1)

	transaction := payloads.Transactions[0]
	require.Equal(t, "GET /articles/find", transaction.Name)
	require.Equal(t, "failure", transaction.Outcome)
	requireKratosFramework(t, transaction)
	require.Equal(t, "GET", transaction.Context.Request.Method)

	fault := payloads.Errors[0]
	require.Equal(t, transaction.ID, fault.TransactionID)
	require.Equal(t, transaction.TraceID, fault.TraceID)
	require.Equal(t, "GET /articles/find", fault.Transaction.Name)
	require.True(t, fault.Exception.Handled)
}

// TestMiddleware_Panic checks a panic turns into 500 INTERNAL_SERVER_ERROR and reaches APM inside the transaction of its request
// APM gets one error, the panic with its stack trace: the 500 error that comes out of it is not sent again
//
// TestMiddleware_Panic 检查 panic 变成 500 INTERNAL_SERVER_ERROR，并在所属请求的事务里上报到 APM
// APM 只收到一条错误，即带调用栈的 panic：由它产生的 500 错误不再重复上报
func TestMiddleware_Panic(t *testing.T) {
	recording := startRecording(t)
	server := newTestServer(t)

	code, body := sendRequest(t, server.URL+"/articles/first", nil)
	require.Equal(t, 500, code)
	require.Equal(t, `{"code":500,"reason":"INTERNAL_SERVER_ERROR","message":"service panic: runtime error: index out of range [0] with length 0"}`, body)

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Len(t, payloads.Errors, 1)

	transaction := payloads.Transactions[0]
	require.Equal(t, "GET /articles/first", transaction.Name)
	require.Equal(t, "failure", transaction.Outcome)
	requireKratosFramework(t, transaction)
	require.Equal(t, "GET", transaction.Context.Request.Method)

	fault := payloads.Errors[0]
	require.Equal(t, transaction.ID, fault.TransactionID)
	require.Equal(t, "GET /articles/first", fault.Transaction.Name)
	require.Equal(t, "runtime error: index out of range [0] with length 0", fault.Exception.Message)
	require.False(t, fault.Exception.Handled)
	require.NotEmpty(t, fault.Exception.Stacktrace)
}

// TestMiddleware_GRPC sends a gRPC request with traceparent to a Kratos gRPC server,
// the health service that each Kratos gRPC server has is the one being called
//
// TestMiddleware_GRPC 向 Kratos gRPC 服务发一个带 traceparent 的 gRPC 请求，调用的是 Kratos gRPC 服务自带的健康检查
func TestMiddleware_GRPC(t *testing.T) {
	recording := startRecording(t)

	srv := kratosgrpc.NewServer(kratosgrpc.Address("127.0.0.1:0"), kratosgrpc.Middleware(Middleware()))
	endpoint, err := srv.Endpoint()
	require.NoError(t, err)
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Start(context.Background())
	}()
	t.Cleanup(func() {
		require.NoError(t, srv.Stop(context.Background()))
		require.NoError(t, <-serveDone)
	})

	conn, err := grpc.NewClient(endpoint.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	ctx := metadata.AppendToOutgoingContext(t.Context(), "traceparent", upstreamTraceparent)
	res, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, res.GetStatus())

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Empty(t, payloads.Errors)

	transaction := payloads.Transactions[0]
	require.Equal(t, "/grpc.health.v1.Health/Check", transaction.Name)
	require.Equal(t, "request", transaction.Type)
	require.Equal(t, "success", transaction.Outcome)
	require.Equal(t, upstreamTraceID, hex.EncodeToString(transaction.TraceID[:]))
	require.Equal(t, upstreamSpanID, hex.EncodeToString(transaction.ParentID[:]))
	requireKratosFramework(t, transaction)
}

// TestNewApmTraceMiddleware_NoTransport checks a context with no server transport passes through, and starts no transaction
// TestNewApmTraceMiddleware_NoTransport 检查没有服务端传输信息的 context 原样放行，也不开事务
func TestNewApmTraceMiddleware_NoTransport(t *testing.T) {
	recording := startRecording(t)

	type ctxMark struct{}
	handle := NewApmTraceMiddleware()(func(ctx context.Context, req any) (any, error) {
		_, hasOutgoing := metadata.FromOutgoingContext(ctx)
		return []any{ctx.Value(ctxMark{}), req, hasOutgoing, apm.TransactionFromContext(ctx) != nil}, nil
	})
	res, err := handle(context.WithValue(t.Context(), ctxMark{}, "kept"), "request")
	require.NoError(t, err)
	require.Equal(t, []any{"kept", "request", false, false}, res)

	recording.Flush(nil)
	require.Empty(t, recording.Payloads().Transactions)
}

// TestNewRecoveryMiddleware checks the panic middleware used with no trace middleware outside:
// the panic turns into 500 INTERNAL_SERVER_ERROR the same as before, and the message says it is not reported
//
// TestNewRecoveryMiddleware 检查外面没有追踪中间件时单用 panic 中间件：
// panic 照样变成 500 INTERNAL_SERVER_ERROR，消息里注明没有上报
func TestNewRecoveryMiddleware(t *testing.T) {
	recording := startRecording(t)

	res, err := NewRecoveryMiddleware()(firstArticle)(t.Context(), nil)
	require.Nil(t, res)
	require.Error(t, err)

	erk := errors.FromError(err)
	require.Equal(t, int32(500), erk.Code)
	require.Equal(t, "INTERNAL_SERVER_ERROR", erk.Reason)
	require.Equal(t, "service panic: runtime error: index out of range [0] with length 0 (not reported, no apm transaction)", erk.Message)

	recording.Flush(nil)
	require.Empty(t, recording.Payloads().Errors)
}

// TestNewRecoveryApmOptions checks efn builds the error that goes back
// TestNewRecoveryApmOptions 检查返回的错误由 efn 构造
func TestNewRecoveryApmOptions(t *testing.T) {
	recording := startRecording(t)

	srv := http.NewServer(http.Middleware(
		NewApmTraceMiddleware(),
		recovery.Recovery(NewRecoveryApmOptions(func(format string, args ...any) *errors.Error {
			return errors.Newf(503, "ARTICLE_SERVICE_PANIC", format, args...)
		})),
	))
	srv.Route("/").GET("/articles/first", func(ctx http.Context) error {
		res, err := ctx.Middleware(firstArticle)(ctx, nil)
		if err != nil {
			return err
		}
		return ctx.Result(200, res)
	})
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)

	code, body := sendRequest(t, server.URL+"/articles/first", nil)
	require.Equal(t, 503, code)
	require.Equal(t, `{"code":503,"reason":"ARTICLE_SERVICE_PANIC","message":"service panic: runtime error: index out of range [0] with length 0"}`, body)

	recording.Flush(nil)
	payloads := recording.Payloads()
	require.Len(t, payloads.Transactions, 1)
	// The trace middleware skips the error that efn builds too: APM gets the panic alone
	// efn 构造的错误同样被追踪中间件跳过：APM 只收到 panic 这一条
	require.Len(t, payloads.Errors, 1)
	require.Equal(t, payloads.Transactions[0].ID, payloads.Errors[0].TransactionID)
	require.Equal(t, "GET /articles/first", payloads.Errors[0].Transaction.Name)
	require.False(t, payloads.Errors[0].Exception.Handled)
}

func TestHTTPTransportFromContext(t *testing.T) {
	// The HTTP case runs inside listArticles in TestMiddleware
	// HTTP 请求的情形在 TestMiddleware 的 listArticles 里
	require.Nil(t, HTTPTransportFromContext(t.Context()))
}
