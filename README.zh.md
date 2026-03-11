[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-apm/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-apm/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-apm)](https://pkg.go.dev/github.com/yylego/kratos-apm)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-apm/main.svg)](https://coveralls.io/github/yylego/kratos-apm?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.25%2B-lightgrey.svg)](https://github.com/yylego/kratos-apm)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-apm.svg)](https://github.com/yylego/kratos-apm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-apm)](https://goreportcard.com/report/github.com/yylego/kratos-apm)

# kratos-apm

<!-- TEMPLATE (ZH) BEGIN: LANGUAGE NAVIGATION -->
## 英文文档

[ENGLISH README](README.md)
<!-- TEMPLATE (ZH) END: LANGUAGE NAVIGATION -->

Kratos 框架的 Elastic APM 中间件，提供分布式追踪和性能监控能力。

## 特性

- 🚀 简单集成 - 只需几行代码即可接入
- 📊 链路追踪 - 自动追踪 gRPC 和 HTTP 请求
- 🔍 错误追踪 - 自动捕获业务错误和 Panic
- 🌐 W3C 标准 - 支持 W3C TraceContext 传播
- ⚡ 零侵入 - 基于 Kratos 中间件实现

## 依赖版本

本项目使用 Elastic APM v2:
```
go.elastic.co/apm/v2
```

不再支持 v1 版本。

## 安装

```bash
go get github.com/yylego/kratos-apm/apmkratos
```

## 快速开始

### 1. 初始化 APM

在应用启动时初始化 APM 配置：

```go
package main

import (
    "github.com/yylego/elasticapm"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // 配置 APM
    apmConfig := &elasticapm.Config{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        ServerURL:      "http://localhost:8200",
    }

    // 初始化 APM
    if err := apmkratos.Initialize(apmConfig); err != nil {
        panic(err)
    }
    defer apmkratos.Close()

    // 启动应用...
}
```

### 2. 集成到 Kratos Server

#### HTTP Server

```go
package main

import (
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/transport/http"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // 创建 HTTP Server，注册 APM 中间件
    httpSrv := http.NewServer(
        http.Address(":8000"),
        http.Middleware(
            apmkratos.Middleware(), // 添加 APM 中间件
        ),
    )

    // 注册服务...

    app := kratos.New(
        kratos.Name("my-service"),
        kratos.Server(httpSrv),
    )

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

#### gRPC Server

```go
package main

import (
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/transport/grpc"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // 创建 gRPC Server，注册 APM 中间件
    grpcSrv := grpc.NewServer(
        grpc.Address(":9000"),
        grpc.Middleware(
            apmkratos.Middleware(), // 添加 APM 中间件
        ),
    )

    // 注册服务...

    app := kratos.New(
        kratos.Name("my-service"),
        kratos.Server(grpcSrv),
    )

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

#### 同时支持 HTTP 和 gRPC

```go
package main

import (
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/transport/grpc"
    "github.com/go-kratos/kratos/v2/transport/http"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // APM 中间件同时支持 HTTP 和 gRPC
    middleware := apmkratos.Middleware()

    httpSrv := http.NewServer(
        http.Address(":8000"),
        http.Middleware(middleware),
    )

    grpcSrv := grpc.NewServer(
        grpc.Address(":9000"),
        grpc.Middleware(middleware),
    )

    // 注册服务...

    app := kratos.New(
        kratos.Name("my-service"),
        kratos.Server(httpSrv, grpcSrv),
    )

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

## 高级用法

### 自定义环境变量配置

```go
package main

import (
    "github.com/yylego/elasticapm"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    apmConfig := &elasticapm.Config{
        ServiceName: "my-service",
        ServerURL:   "http://apm-server:8200",
    }

    // 自定义环境变量选项
    envOption := elasticapm.NewEnvOption()

    // 使用自定义选项初始化
    if err := apmkratos.InitializeWithOptions(apmConfig, envOption); err != nil {
        panic(err)
    }
    defer apmkratos.Close()
}
```

### 版本对齐检查

确保使用 APM 的模块版本保持对齐：

```go
package main

import (
    "github.com/yylego/kratos-apm/apmkratos"
    "go.elastic.co/apm/v2"
)

func main() {
    // 检查 APM 版本
    version := apmkratos.GetApmAgentVersion()
    println("APM Agent Version:", version)

    // 检查版本对齐
    if !apmkratos.CheckApmAgentVersion(apm.AgentVersion) {
        panic("APM version mismatch")
    }
}
```

### 完整示例

```go
package main

import (
    "context"

    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/log"
    "github.com/go-kratos/kratos/v2/middleware/recovery"
    "github.com/go-kratos/kratos/v2/transport/grpc"
    "github.com/go-kratos/kratos/v2/transport/http"
    "github.com/yylego/elasticapm"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // 1. 初始化 APM
    apmConfig := &elasticapm.Config{
        ServiceName:    "demo-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        ServerURL:      "http://localhost:8200",
    }

    if err := apmkratos.Initialize(apmConfig); err != nil {
        log.Fatal(err)
    }
    defer apmkratos.Close()

    // 2. 创建 HTTP Server
    httpSrv := http.NewServer(
        http.Address(":8000"),
        http.Middleware(
            recovery.Recovery(), // 建议配合 recovery 中间件
            apmkratos.Middleware(),
        ),
    )

    // 3. 创建 gRPC Server
    grpcSrv := grpc.NewServer(
        grpc.Address(":9000"),
        grpc.Middleware(
            recovery.Recovery(),
            apmkratos.Middleware(),
        ),
    )

    // 4. 注册服务处理器
    // RegisterGreeterHTTPServer(httpSrv, &GreeterService{})
    // RegisterGreeterServer(grpcSrv, &GreeterService{})

    // 5. 启动应用
    app := kratos.New(
        kratos.Name("demo-service"),
        kratos.Version("1.0.0"),
        kratos.Server(httpSrv, grpcSrv),
    )

    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}

// GreeterService 示例服务
type GreeterService struct{}

func (s *GreeterService) SayHello(ctx context.Context, req *HelloRequest) (*HelloReply, error) {
    // 业务逻辑
    // APM 自动追踪此请求
    return &HelloReply{Message: "Hello " + req.Name}, nil
}
```

## 中间件功能

### 自动追踪

APM 中间件自动追踪：

- ✅ 请求的完整链路
- ✅ 请求耗时和性能指标
- ✅ 上下文传播 (W3C TraceContext)
- ✅ 框架信息 (Kratos v2)

### 错误追踪

自动捕获和上报：

- ✅ 业务错误 (通过 middleware.Handler 返回)
- ✅ Panic 异常 (配合 recovery 中间件)
- ✅ 错误堆栈和上下文信息

### 上下文传播

支持跨服务的链路追踪：

```go
// 服务 A 调用服务 B，追踪信息自动传播
func (s *ServiceA) CallServiceB(ctx context.Context) error {
    // ctx 中包含追踪信息
    // APM 自动添加 TraceContext 到请求头
    resp, err := s.serviceBClient.DoSomething(ctx, &Request{})
    return err
}
```

### 访问 HTTP Transport

需要时从上下文获取 HTTP transport：

```go
import "github.com/yylego/kratos-apm/apmkratos"

func MyHandler(ctx context.Context, req *Request) (*Response, error) {
    // 获取 HTTP transport（如果可用）
    transport := apmkratos.GetHttpTransportFromContext(ctx)
    if transport != nil {
        // 访问请求信息
        httpReq := transport.Request()
        // 处理 HTTP 请求
    }
    return &Response{}, nil
}
```

## 配置说明

### APM Config 参数

```go
type Config struct {
    ServiceName    string // 服务名称 (必填)
    ServiceVersion string // 服务版本
    Environment    string // 环境标识 (dev/staging/production)
    ServerURL      string // APM Server 地址
}
```

### 环境变量

也可以通过环境变量配置 APM：

```bash
export ELASTIC_APM_SERVICE_NAME="my-service"
export ELASTIC_APM_SERVER_URL="http://localhost:8200"
export ELASTIC_APM_ENVIRONMENT="production"
export ELASTIC_APM_SERVICE_VERSION="1.0.0"
```

## 最佳实践

### 1. 中间件顺序

建议将 APM 中间件放置在业务中间件之后、recovery 中间件之前：

```go
http.Middleware(
    logging.Server(),      // 日志中间件
    recovery.Recovery(),   // 恢复中间件
    apmkratos.Middleware(), // APM 中间件
    validate.Validator(),  // 验证中间件
)
```

### 2. 与 Recovery 中间件配合

APM 中间件内部已经集成了 Recovery 功能，如果单独使用可以：

```go
// 方式 1：使用 APM 自带的 Recovery
http.Middleware(
    apmkratos.Middleware(), // 已包含 Recovery
)

// 方式 2：使用 Kratos 的 Recovery + APM
http.Middleware(
    recovery.Recovery(),
    apmkratos.Middleware(),
)
```

### 3. 版本检查

在应用启动时检查 APM 版本对齐：

```go
func init() {
    version := apmkratos.GetApmAgentVersion()
    log.Infof("APM Agent Version: %s", version)

    if !apmkratos.CheckApmAgentVersion(apm.AgentVersion) {
        log.Warn("APM version mismatch detected")
    }
}
```

## 依赖项

- `github.com/go-kratos/kratos/v2` - Kratos 微服务框架
- `go.elastic.co/apm/v2` - Elastic APM Go Agent
- `github.com/yylego/elasticapm` - APM 配置辅助包
- `github.com/yylego/zaplog` - 日志工具

## 相关项目

- [Kratos](https://github.com/go-kratos/kratos) - Go 微服务框架
- [Elastic APM](https://www.elastic.co/apm) - 应用性能监控
- [elasticapm](https://github.com/yylego/elasticapm) - APM 配置工具

<!-- TEMPLATE (ZH) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-09-26 07:39:27.188023 +0000 UTC -->

## 📄 许可证

MIT License - 查看 [LICENSE](LICENSE) 文件

---

## 💬 联系反馈

**问题和反馈：**

- 🐛 **Bug 报告？** 打开 issue 并描述问题和复现步骤
- ✨ **功能想法？** 打开 issue 讨论实现方案
- 📖 **文档疑惑？** 报告问题，帮助我们改进文档
- 🚀 **需要功能？** 分享使用场景，帮助理解需求
- ⚡ **性能瓶颈？** 报告慢操作，帮助我们优化性能
- 🔧 **配置困扰？** 询问复杂设置的相关问题
- 📢 **关注进展？** 关注仓库以获取新版本和功能
- 🌟 **成功案例？** 分享这个包如何改善工作流程
- 💬 **反馈意见？** 欢迎提出建议和意见

---

## 🔧 代码贡献

新代码贡献，请遵循此流程：

1. **Fork**：在 GitHub 上 Fork 仓库（使用网页界面）
2. **克隆**：克隆 Fork 的项目（`git clone https://github.com/yourname/kratos-apm.git`）
3. **导航**：进入克隆的项目（`cd kratos-apm`）
4. **分支**：创建功能分支（`git checkout -b feature/xxx`）
5. **编码**：实现您的更改并编写全面的测试
6. **测试**：（Golang 项目）确保测试通过（`go test ./...`）并遵循 Go 代码风格约定
7. **文档**：为面向用户的更改更新文档，并使用有意义的提交消息
8. **暂存**：暂存更改（`git add .`）
9. **提交**：提交更改（`git commit -m "Add feature xxx"`）确保向后兼容的代码
10. **推送**：推送到分支（`git push origin feature/xxx`）
11. **PR**：在 GitHub 上打开 Merge Request（在 GitHub 网页上）并提供详细描述

请确保测试通过并包含相关的文档更新。

---

## 🌟 项目支持

非常欢迎通过提交 Merge Request 和报告问题来为此项目做出贡献。

**项目支持：**

- ⭐ **给予星标**如果项目对您有帮助
- 🤝 **分享项目**给团队成员和（golang）编程朋友
- 📝 **撰写博客**关于开发工具和工作流程 - 我们提供写作支持
- 🌟 **加入生态** - 致力于支持开源和（golang）开发场景

**祝你用这个包编程愉快！** 🎉🎉🎉

<!-- TEMPLATE (ZH) END: STANDARD PROJECT FOOTER -->

---

## GitHub 标星点赞

[![标星点赞](https://starchart.cc/yylego/kratos-apm.svg?variant=adaptive)](https://starchart.cc/yylego/kratos-apm)
