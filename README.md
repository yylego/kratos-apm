<!-- TEMPLATE (EN) BEGIN: BADGES -->

[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-apm/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-apm/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-apm)](https://pkg.go.dev/github.com/yylego/kratos-apm)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-apm/main.svg)](https://coveralls.io/github/yylego/kratos-apm?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26%2B-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-apm.svg)](https://github.com/yylego/kratos-apm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-apm)](https://goreportcard.com/report/github.com/yylego/kratos-apm)

<!-- TEMPLATE (EN) CLOSE: BADGES -->

# kratos-apm

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->

## CHINESE README

[中文说明](README.zh.md)

<!-- TEMPLATE (EN) CLOSE: LANGUAGE NAVIGATION -->

Kratos middleware that reports to Elastic APM: one APM transaction on each gRPC and HTTP request, with errors and panics reported to APM.

## Features

- One middleware serves both the Kratos HTTP server and the Kratos gRPC server
- Each request becomes an APM transaction, marked with the framework `kratos v3`
- The W3C `traceparent` and `tracestate` of the request continue the upstream trace
- The same trace context goes into the outgoing gRPC metadata, so downstream gRPC services join the trace
- An error from the handle func is reported to APM inside the transaction of its request
- A panic is recovered, reported to APM, and answered with `500 INTERNAL_SERVER_ERROR`

## Version

This project uses Elastic APM v2 and Kratos v3:

```
go.elastic.co/apm/v2
github.com/go-kratos/kratos/v3
```

## Installation

```bash
go get github.com/yylego/kratos-apm/apmkratos
```

## Quick Start

Start the APM agent when the application starts, and give `apmkratos.Middleware()` to the servers:

```go
package main

import (
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/yylego/elasticapm"
	"github.com/yylego/kratos-apm/apmkratos"
)

func main() {
	// Start the APM agent, and send what is left when the application exits
	if err := apmkratos.Initialize(&elasticapm.Config{
		Environment:    "production",
		ServerUrl:      "http://localhost:8200",
		ServiceName:    "article-service",
		ServiceVersion: "1.0.0",
	}); err != nil {
		panic(err)
	}
	defer apmkratos.Close()

	// The same middleware works on both servers
	httpSrv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(apmkratos.Middleware()),
	)
	grpcSrv := grpc.NewServer(
		grpc.Address(":9000"),
		grpc.Middleware(apmkratos.Middleware()),
	)

	// Register the services on httpSrv and grpcSrv here

	app := kratos.New(
		kratos.Name("article-service"),
		kratos.Version("1.0.0"),
		kratos.Server(httpSrv, grpcSrv),
	)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
```

An application with one kind of server passes the middleware to that server alone.

## Middleware Actions

`apmkratos.Middleware()` is two middlewares in one: the trace middleware outside, the panic middleware inside.

### Transactions

| Request | Transaction name                  | Example                                |
| ------- | --------------------------------- | -------------------------------------- |
| HTTP    | request method and route template | `GET /articles/{id}`                   |
| gRPC    | Kratos operation                  | `/api.article.v1.Article/ListArticles` |

The transaction type is `request`. An HTTP transaction also carries the request (method, URL, headers).

The HTTP response status code is not recorded: Kratos writes the response once the middleware has returned.

### Errors and Panics

- When the handle func returns an error, the error goes back to the client unchanged, and APM gets it as a handled error of the transaction.
- When the handle func panics, the client gets `500 INTERNAL_SERVER_ERROR` with the message `service panic: <panic value>`.
- APM gets one error on a panic, the panic with its stack trace. The 500 error that comes out of it is not reported once more.

### Trace Propagation

A request that carries `traceparent`, in the HTTP headers and in the gRPC metadata alike, continues the trace of the upstream service. A request without it starts a new trace.

The middleware puts the trace context into the outgoing gRPC metadata of the context. Pass that context on when calling a downstream gRPC service:

```go
func (s *ArticleService) ListArticles(ctx context.Context, req *v1.ListArticlesRequest) (*v1.ListArticlesReply, error) {
	// ctx carries traceparent and tracestate, the downstream service continues the same trace
	comments, err := s.commentClient.ListComments(ctx, &commentv1.ListCommentsRequest{})
	if err != nil {
		return nil, err
	}
	return &v1.ListArticlesReply{Comments: comments.GetComments()}, nil
}
```

Outgoing HTTP requests are outside the scope of this package. Use `apmhttp.WrapClient` from `go.elastic.co/apm/module/apmhttp/v2` on the HTTP client, and send the request with the same context.

## Middleware Sequence

Put `apmkratos.Middleware()` first. The first middleware is the outermost one in Kratos, so the transaction then covers the time of the middlewares behind it, and the errors those middlewares return are reported too:

```go
http.Middleware(
	apmkratos.Middleware(),         // Trace and panic handling
	logging.Server(slog.Default()), // The rest of the middlewares come next
)
```

`apmkratos.Middleware()` handles panics, so there is no need to add `recovery.Recovery()` from Kratos once more.

## Custom Panic Response

The two parts are also available one at a time. `NewRecoveryApmOptions` gives the APM part of the panic handling, with the error to respond built in a func:

```go
http.Middleware(
	apmkratos.NewApmTraceMiddleware(), // Has to be outside the panic middleware
	recovery.Recovery(apmkratos.NewRecoveryApmOptions(func(format string, args ...any) *errors.Error {
		return errors.Newf(503, "ARTICLE_SERVICE_PANIC", format, args...)
	})),
)
```

Here `recovery` is `github.com/go-kratos/kratos/v3/middleware/recovery`, and `errors` is `github.com/go-kratos/kratos/v3/errors`.

`apmkratos.NewRecoveryMiddleware()` is the same thing with the default `500 INTERNAL_SERVER_ERROR` response.

With no trace middleware outside, the panic gets its response the same as before, but it is not reported to APM, and the message says so.

## Environment Options

The Elastic APM agent reads its config from `ELASTIC_APM_*` environment variables. `Initialize` writes the config into those variables, replacing the values that are set, then starts the agent.

`InitializeWithOptions` gives two more controls:

- `EnvOption{Override: false}` keeps the variables that are set, the config fills in the others.
- Each func passed at the end runs once the config is in the environment, ahead of starting the agent. It is the place to set more agent options.

```go
err := apmkratos.InitializeWithOptions(apmConfig, &elasticapm.EnvOption{Override: false}, func() {
	if err := os.Setenv("ELASTIC_APM_TRANSACTION_SAMPLE_RATE", "0.5"); err != nil {
		panic(err)
	}
})
```

`Initialize` also sends the logs of the agent itself to zap. `InitializeWithOptions` leaves the agent logs unchanged.

## Config Fields

The config is `elasticapm.Config` from [elasticapm](https://github.com/yylego/elasticapm):

```go
type Config struct {
	Environment    string   // Environment name, such as "production"
	ServerUrls     []string // Addresses of APM servers, when there are more than one
	ServerUrl      string   // Address of the APM server
	ApiKey         string   // API key
	SecretToken    string   // Secret token
	ServiceName    string   // Service name
	ServiceVersion string   // Service version
	NodeName       string   // Node name, when the service runs on more than one node
	ServerCertPath string   // Path of the TLS certificate
	SkipShortSpans bool     // When false, each span keeps its stack trace, short spans included
}
```

One of `ServerUrl` and `ServerUrls` has to be set.

## Agent Version Check

`go.elastic.co/apm` (v1) and `go.elastic.co/apm/v2` are two packages with two separate agents. A transaction started with one of them is unknown to the second one, so an application that imports v1 does not see the transactions of this middleware. Check it at startup:

```go
// apm is the agent package that the application imports
if !apmkratos.CheckApmAgentVersion(apm.AgentVersion) {
	panic("apm agent version mismatch, this package uses " + apmkratos.ApmAgentVersion())
}
```

## HTTP Transport Access

`HTTPTransportFromContext` returns the Kratos HTTP transport of the request, and nil when the request is a gRPC one:

```go
func (s *ArticleService) ListArticles(ctx context.Context, req *v1.ListArticlesRequest) (*v1.ListArticlesReply, error) {
	if httpTransport := apmkratos.HTTPTransportFromContext(ctx); httpTransport != nil {
		userAgent := httpTransport.Request().UserAgent()
		// Use the HTTP request here
		_ = userAgent
	}
	return &v1.ListArticlesReply{}, nil
}
```

## Related Projects

- [Kratos](https://github.com/go-kratos/kratos) - Go microservice framework
- [Elastic APM Go agent](https://github.com/elastic/apm-agent-go) - The agent this package is built on
- [elasticapm](https://github.com/yylego/elasticapm) - Config and startup of the agent

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-11-25 03:52:28.131064 +0000 UTC -->

## 📄 License

MIT License - see [LICENSE](LICENSE).

---

## 💬 Contact & Feedback

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Mistake reports?** Open an issue on GitHub with reproduction steps
- 💡 **Fresh ideas?** Create an issue to discuss
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a merge request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉🎉🎉

<!-- TEMPLATE (EN) CLOSE: STANDARD PROJECT FOOTER -->
