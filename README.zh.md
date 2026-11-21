<!-- TEMPLATE (ZH) BEGIN: BADGES -->

[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-apm/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-apm/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-apm)](https://pkg.go.dev/github.com/yylego/kratos-apm)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-apm/main.svg)](https://coveralls.io/github/yylego/kratos-apm?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26%2B-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-apm.svg)](https://github.com/yylego/kratos-apm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-apm)](https://goreportcard.com/report/github.com/yylego/kratos-apm)

<!-- TEMPLATE (ZH) CLOSE: BADGES -->

# kratos-apm

<!-- TEMPLATE (ZH) BEGIN: LANGUAGE NAVIGATION -->

## 英文文档

[ENGLISH README](README.md)

<!-- TEMPLATE (ZH) CLOSE: LANGUAGE NAVIGATION -->

Kratos 框架的 Elastic APM 中间件：为每个 gRPC、HTTP 请求开一个 APM 事务，并把错误和 panic 上报到 APM。

## 功能特性

- 一个中间件同时用于 Kratos 的 HTTP 服务和 gRPC 服务
- 每个请求对应一个 APM 事务，框架信息标为 `kratos v3`
- 读取请求带来的 W3C `traceparent` 和 `tracestate`，接上上游的链路
- 同一份链路信息会写进发往下游的 gRPC metadata，下游的 gRPC 服务接着同一条链路
- 处理函数返回的错误，在所属请求的事务里上报到 APM
- 发生 panic 时捕获并上报到 APM，返回 `500 INTERNAL_SERVER_ERROR`

## 依赖版本

本项目使用 Elastic APM v2 和 Kratos v3：

```
go.elastic.co/apm/v2
github.com/go-kratos/kratos/v3
```

## 安装方式

```bash
go get github.com/yylego/kratos-apm/apmkratos
```

## 快速开始

在应用启动时启动 APM agent，再把 `apmkratos.Middleware()` 传给服务：

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
	// 启动 APM agent，应用退出时把剩下的数据发完
	if err := apmkratos.Initialize(&elasticapm.Config{
		Environment:    "production",
		ServerUrl:      "http://localhost:8200",
		ServiceName:    "article-service",
		ServiceVersion: "1.0.0",
	}); err != nil {
		panic(err)
	}
	defer apmkratos.Close()

	// 同一个中间件在两种服务上都能用
	httpSrv := http.NewServer(
		http.Address(":8000"),
		http.Middleware(apmkratos.Middleware()),
	)
	grpcSrv := grpc.NewServer(
		grpc.Address(":9000"),
		grpc.Middleware(apmkratos.Middleware()),
	)

	// 在这里把业务服务注册到 httpSrv 和 grpcSrv

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

只有一种服务的应用，把中间件传给那一种服务即可。

## 中间件行为

`apmkratos.Middleware()` 由两个中间件组成：追踪中间件在外，panic 中间件在内。

### 事务信息

| 请求 | 事务名称            | 示例                                   |
| ---- | ------------------- | -------------------------------------- |
| HTTP | 请求方法加路由模板  | `GET /articles/{id}`                   |
| gRPC | Kratos 的 operation | `/api.article.v1.Article/ListArticles` |

事务类型是 `request`。HTTP 请求的事务还带着请求信息（方法、URL、请求头）。

HTTP 响应状态码没有记录：Kratos 在中间件返回之后才写响应。

### 错误与 panic

- 处理函数返回错误时，错误原样返回给客户端，同时作为这次事务里已处理的错误上报到 APM。
- 处理函数发生 panic 时，客户端收到 `500 INTERNAL_SERVER_ERROR`，消息是 `service panic: <panic 的值>`。
- 一次 panic 在 APM 里只有一条错误，即带调用栈的 panic。由它产生的那个 500 错误不再重复上报。

### 链路传播

请求带着 `traceparent` 请求头（HTTP）或 metadata（gRPC）时，接上上游服务的链路；不带时从这里开始一条新链路。

中间件把链路信息写进 context 里发往下游的 gRPC metadata。调用下游 gRPC 服务时把这个 context 传下去：

```go
func (s *ArticleService) ListArticles(ctx context.Context, req *v1.ListArticlesRequest) (*v1.ListArticlesReply, error) {
	// ctx 里带着 traceparent 和 tracestate，下游服务接着同一条链路
	comments, err := s.commentClient.ListComments(ctx, &commentv1.ListCommentsRequest{})
	if err != nil {
		return nil, err
	}
	return &v1.ListArticlesReply{Comments: comments.GetComments()}, nil
}
```

发往下游的 HTTP 请求不在本包的范围内。用 `go.elastic.co/apm/module/apmhttp/v2` 的 `apmhttp.WrapClient` 包装 HTTP 客户端，并带着同一个 context 发请求。

## 中间件顺序

把 `apmkratos.Middleware()` 放在第一个。Kratos 里第一个中间件在最外层，这样事务的耗时包含后面各个中间件，它们返回的错误也会上报：

```go
http.Middleware(
	apmkratos.Middleware(),         // 追踪和 panic 处理
	logging.Server(slog.Default()), // 其他中间件排在后面
)
```

`apmkratos.Middleware()` 已经处理 panic，不需要再加一次 Kratos 的 `recovery.Recovery()`。

## 自定义 panic 响应

两个部分也可以分开用。`NewRecoveryApmOptions` 提供 panic 处理里上报 APM 的那一部分，要返回的错误由传入的函数构造：

```go
http.Middleware(
	apmkratos.NewApmTraceMiddleware(), // 要放在 panic 中间件的外面
	recovery.Recovery(apmkratos.NewRecoveryApmOptions(func(format string, args ...any) *errors.Error {
		return errors.Newf(503, "ARTICLE_SERVICE_PANIC", format, args...)
	})),
)
```

这里的 `recovery` 是 `github.com/go-kratos/kratos/v3/middleware/recovery`，`errors` 是 `github.com/go-kratos/kratos/v3/errors`。

`apmkratos.NewRecoveryMiddleware()` 是同一个东西，响应固定为默认的 `500 INTERNAL_SERVER_ERROR`。

外面没有追踪中间件时，panic 照样有响应，但不会上报到 APM，返回的消息里会注明。

## 环境变量选项

Elastic APM agent 从 `ELASTIC_APM_*` 环境变量读取配置。`Initialize` 把配置写进这些环境变量，已有的值会被覆盖，然后启动 agent。

`InitializeWithOptions` 多出两处可以控制：

- `EnvOption{Override: false}` 保留已经设置的环境变量，配置只填上还没设置的。
- 最后传入的每个函数，在配置写进环境变量之后、启动 agent 之前执行一次，可以在里面设置更多 agent 选项。

```go
err := apmkratos.InitializeWithOptions(apmConfig, &elasticapm.EnvOption{Override: false}, func() {
	if err := os.Setenv("ELASTIC_APM_TRANSACTION_SAMPLE_RATE", "0.5"); err != nil {
		panic(err)
	}
})
```

`Initialize` 还会把 agent 自身的日志接到 zap。`InitializeWithOptions` 不改动 agent 的日志。

## 配置字段

配置用的是 [elasticapm](https://github.com/yylego/elasticapm) 的 `elasticapm.Config`：

```go
type Config struct {
	Environment    string   // 环境名称，如 "production"
	ServerUrls     []string // 多个 APM 服务地址
	ServerUrl      string   // 单个 APM 服务地址
	ApiKey         string   // API 密钥
	SecretToken    string   // Secret Token
	ServiceName    string   // 服务名称
	ServiceVersion string   // 服务版本
	NodeName       string   // 节点名称，服务跑在多个节点上时使用
	ServerCertPath string   // TLS 证书路径
	SkipShortSpans bool     // 为 false 时，每个 span 都保留调用栈，短 span 也不例外
}
```

`ServerUrl` 和 `ServerUrls` 至少要填一个。

## 版本一致检查

`go.elastic.co/apm`（v1）和 `go.elastic.co/apm/v2` 是两个包，各有一个独立的 agent。用其中一个开启的事务，另一个并不知道，所以引入 v1 的应用看不到本中间件开启的事务。在启动时检查：

```go
// apm 是应用自己引入的 agent 包
if !apmkratos.CheckApmAgentVersion(apm.AgentVersion) {
	panic("apm agent version mismatch, this package uses " + apmkratos.ApmAgentVersion())
}
```

## 获取 HTTP 传输对象

`HTTPTransportFromContext` 返回这次请求的 Kratos HTTP 传输对象，请求是 gRPC 请求时返回 nil：

```go
func (s *ArticleService) ListArticles(ctx context.Context, req *v1.ListArticlesRequest) (*v1.ListArticlesReply, error) {
	if httpTransport := apmkratos.HTTPTransportFromContext(ctx); httpTransport != nil {
		userAgent := httpTransport.Request().UserAgent()
		// 在这里使用 HTTP 请求
		_ = userAgent
	}
	return &v1.ListArticlesReply{}, nil
}
```

## 相关项目

- [Kratos](https://github.com/go-kratos/kratos) - Go 微服务框架
- [Elastic APM Go agent](https://github.com/elastic/apm-agent-go) - 本包所依赖的 agent
- [elasticapm](https://github.com/yylego/elasticapm) - agent 的配置与启动

<!-- TEMPLATE (ZH) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-11-25 03:52:28.131064 +0000 UTC -->

## 📄 许可证类型

MIT 许可证 - 详见 [LICENSE](LICENSE)。

---

## 💬 联系与反馈

非常欢迎贡献代码！报告 BUG、建议功能、贡献代码：

- 🐛 **问题报告？** 在 GitHub 上提交问题并附上重现步骤
- 💡 **新颖思路？** 创建 issue 讨论
- 📖 **文档疑惑？** 报告问题，帮助我们完善文档
- 🚀 **需要功能？** 分享使用场景，帮助理解需求
- ⚡ **性能瓶颈？** 报告慢操作，协助解决性能问题
- 🔧 **配置困扰？** 询问复杂设置的相关问题
- 📢 **关注进展？** 关注仓库以获取新版本和功能
- 🌟 **成功案例？** 分享这个包如何改善工作流程
- 💬 **反馈意见？** 欢迎提出建议和意见

---

## 🔧 代码贡献

新代码贡献，请遵循此流程：

1. **Fork**：在 GitHub 上 Fork 仓库（使用网页界面）
2. **克隆**：克隆 Fork 的项目（`git clone https://github.com/yourname/repo-name.git`）
3. **导航**：进入克隆的项目（`cd repo-name`）
4. **分支**：创建功能分支（`git checkout -b feature/xxx`）
5. **编码**：实现您的更改并编写全面的测试
6. **测试**：（Golang 项目）确保测试通过（`go test ./...`）并遵循 Go 代码风格约定
7. **文档**：面向用户的更改需要更新文档
8. **暂存**：暂存更改（`git add .`）
9. **提交**：提交更改（`git commit -m "Add feature xxx"`）确保向后兼容的代码
10. **推送**：推送到分支（`git push origin feature/xxx`）
11. **PR**：在 GitHub 上打开 Merge Request（在 GitHub 网页上）并提供详细描述

请确保测试通过并包含相关的文档更新。

---

## 🌟 项目支持

非常欢迎通过提交 Merge Request 和报告问题来贡献此项目。

**项目支持：**

- ⭐ **给予星标**如果项目对您有帮助
- 🤝 **分享项目**给团队成员和（golang）编程朋友
- 📝 **撰写博客**关于开发工具和工作流程 - 我们提供写作支持
- 🌟 **加入生态** - 致力于支持开源和（golang）开发场景

**祝你用这个包编程愉快！** 🎉🎉🎉

<!-- TEMPLATE (ZH) CLOSE: STANDARD PROJECT FOOTER -->
