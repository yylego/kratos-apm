[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-apm/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-apm/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-apm)](https://pkg.go.dev/github.com/yylego/kratos-apm)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-apm/main.svg)](https://coveralls.io/github/yylego/kratos-apm?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.25%2B-lightgrey.svg)](https://github.com/yylego/kratos-apm)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-apm.svg)](https://github.com/yylego/kratos-apm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-apm)](https://goreportcard.com/report/github.com/yylego/kratos-apm)

# kratos-apm

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->
## CHINESE README

[中文说明](README.zh.md)
<!-- TEMPLATE (EN) END: LANGUAGE NAVIGATION -->

Elastic APM middleware to integrate with Kratos framework, providing distributed tracing and performance monitoring.

## Features

- 🚀 Simple Integration - Get started with just a few lines of code
- 📊 Distributed Tracing - Auto trace gRPC and HTTP requests
- 🔍 Error Tracking - Auto capture business errors and panics
- 🌐 W3C Standard - Support W3C TraceContext propagation
- ⚡ Zero Intrusion - Built on Kratos middleware

## Version

This project uses Elastic APM v2:
```
go.elastic.co/apm/v2
```

## Installation

```bash
go get github.com/yylego/kratos-apm/apmkratos
```

## Quick Start

### 1. Initialize APM

Initialize APM config when application starts:

```go
package main

import (
    "github.com/yylego/elasticapm"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // Config APM
    apmConfig := &elasticapm.Config{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        ServerURL:      "http://localhost:8200",
    }

    // Initialize APM
    if err := apmkratos.Initialize(apmConfig); err != nil {
        panic(err)
    }
    defer apmkratos.Close()

    // Start app...
}
```

### 2. Integrate with Kratos Server

#### HTTP Server

```go
package main

import (
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/transport/http"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // Create HTTP Server with APM middleware
    httpSrv := http.NewServer(
        http.Address(":8000"),
        http.Middleware(
            apmkratos.Middleware(), // Add APM middleware
        ),
    )

    // Register services...

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
    // Create gRPC Server with APM middleware
    grpcSrv := grpc.NewServer(
        grpc.Address(":9000"),
        grpc.Middleware(
            apmkratos.Middleware(), // Add APM middleware
        ),
    )

    // Register services...

    app := kratos.New(
        kratos.Name("my-service"),
        kratos.Server(grpcSrv),
    )

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

#### Both HTTP and gRPC

```go
package main

import (
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/transport/grpc"
    "github.com/go-kratos/kratos/v2/transport/http"
    "github.com/yylego/kratos-apm/apmkratos"
)

func main() {
    // APM middleware supports both HTTP and gRPC
    middleware := apmkratos.Middleware()

    httpSrv := http.NewServer(
        http.Address(":8000"),
        http.Middleware(middleware),
    )

    grpcSrv := grpc.NewServer(
        grpc.Address(":9000"),
        grpc.Middleware(middleware),
    )

    // Register services...

    app := kratos.New(
        kratos.Name("my-service"),
        kratos.Server(httpSrv, grpcSrv),
    )

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

## Advanced Usage

### Custom Environment Config

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

    // Custom environment option
    envOption := elasticapm.NewEnvOption()

    // Initialize with custom option
    if err := apmkratos.InitializeWithOptions(apmConfig, envOption); err != nil {
        panic(err)
    }
    defer apmkratos.Close()
}
```

### Version Alignment Check

Ensure modules using APM maintain version alignment:

```go
package main

import (
    "github.com/yylego/kratos-apm/apmkratos"
    "go.elastic.co/apm/v2"
)

func main() {
    // Check APM version
    version := apmkratos.GetApmAgentVersion()
    println("APM Agent Version:", version)

    // Check version alignment
    if !apmkratos.CheckApmAgentVersion(apm.AgentVersion) {
        panic("APM version mismatch")
    }
}
```

### Complete Example

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
    // 1. Initialize APM
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

    // 2. Create HTTP Server
    httpSrv := http.NewServer(
        http.Address(":8000"),
        http.Middleware(
            recovery.Recovery(), // Recommend to work with recovery middleware
            apmkratos.Middleware(),
        ),
    )

    // 3. Create gRPC Server
    grpcSrv := grpc.NewServer(
        grpc.Address(":9000"),
        grpc.Middleware(
            recovery.Recovery(),
            apmkratos.Middleware(),
        ),
    )

    // 4. Register service handlers
    // RegisterGreeterHTTPServer(httpSrv, &GreeterService{})
    // RegisterGreeterServer(grpcSrv, &GreeterService{})

    // 5. Start app
    app := kratos.New(
        kratos.Name("demo-service"),
        kratos.Version("1.0.0"),
        kratos.Server(httpSrv, grpcSrv),
    )

    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}

// GreeterService example service
type GreeterService struct{}

func (s *GreeterService) SayHello(ctx context.Context, req *HelloRequest) (*HelloReply, error) {
    // Business logic
    // APM auto traces this request
    return &HelloReply{Message: "Hello " + req.Name}, nil
}
```

## Middleware Features

### Auto Tracing

APM middleware auto traces:

- ✅ Complete request trace
- ✅ Request duration and performance metrics
- ✅ Context propagation (W3C TraceContext)
- ✅ Framework info (Kratos v2)

### Error Tracking

Auto capture and report:

- ✅ Business errors (from middleware.Handler)
- ✅ Panic exceptions (works with recovery middleware)
- ✅ Error stack and context info

### Context Propagation

Support cross-service distributed tracing:

```go
// Service A calls Service B, trace info auto propagates
func (s *ServiceA) CallServiceB(ctx context.Context) error {
    // ctx contains trace info
    // APM auto adds TraceContext to request headers
    resp, err := s.serviceBClient.DoSomething(ctx, &Request{})
    return err
}
```

### Access HTTP Transport

Get HTTP transport from context when needed:

```go
import "github.com/yylego/kratos-apm/apmkratos"

func MyHandler(ctx context.Context, req *Request) (*Response, error) {
    // Get HTTP transport if available
    transport := apmkratos.GetHttpTransportFromContext(ctx)
    if transport != nil {
        // Access request info
        httpReq := transport.Request()
        // Do something with HTTP request
    }
    return &Response{}, nil
}
```

## Configuration

### APM Config Parameters

```go
type Config struct {
    ServiceName    string // Service name (required)
    ServiceVersion string // Service version
    Environment    string // Environment (dev/staging/production)
    ServerURL      string // APM Server address
}
```

### Environment Variables

You can also config APM via environment variables:

```bash
export ELASTIC_APM_SERVICE_NAME="my-service"
export ELASTIC_APM_SERVER_URL="http://localhost:8200"
export ELASTIC_APM_ENVIRONMENT="production"
export ELASTIC_APM_SERVICE_VERSION="1.0.0"
```

## Best Practices

### 1. Middleware Sequence

Recommend placing APM middleware following business middlewares and preceding panic recovery middleware:

```go
http.Middleware(
    logging.Server(),      // Logging middleware
    recovery.Recovery(),   // Recovery middleware
    apmkratos.Middleware(), // APM middleware
    validate.Validator(),  // Validation middleware
)
```

### 2. Work with Recovery Middleware

APM middleware has integrated Recovery feature, you can use it in two ways:

```go
// Way 1: Use APM's built-in Recovery
http.Middleware(
    apmkratos.Middleware(), // Already includes Recovery
)

// Way 2: Use Kratos Recovery + APM
http.Middleware(
    recovery.Recovery(),
    apmkratos.Middleware(),
)
```

### 3. Version Check

Check APM version alignment when app starts:

```go
func init() {
    version := apmkratos.GetApmAgentVersion()
    log.Infof("APM Agent Version: %s", version)

    if !apmkratos.CheckApmAgentVersion(apm.AgentVersion) {
        log.Warn("APM version mismatch detected")
    }
}
```

## Dependencies

- `github.com/go-kratos/kratos/v2` - Kratos microservice framework
- `go.elastic.co/apm/v2` - Elastic APM Go Agent
- `github.com/yylego/elasticapm` - APM config helper
- `github.com/yylego/zaplog` - Logging tool

## Related Projects

- [Kratos](https://github.com/go-kratos/kratos) - Go microservice framework
- [Elastic APM](https://www.elastic.co/apm) - Application Performance Monitoring
- [elasticapm](https://github.com/yylego/elasticapm) - APM config tool

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-09-26 07:39:27.188023 +0000 UTC -->

## 📄 License

MIT License - see [LICENSE](LICENSE) file

---

## 💬 Contact & Feedback

**Issues & Feedback:**

- 🐛 **Bug reports?** Open an issue and describe the problem with reproduction steps
- ✨ **Feature ideas?** Open an issue to discuss the implementation approach
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
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/kratos-apm.git`).
3. **Navigate**: Navigate to the cloned project (`cd kratos-apm`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes and use significant commit messages
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

<!-- TEMPLATE (EN) END: STANDARD PROJECT FOOTER -->

---

## GitHub Stars

[![Stargazers](https://starchart.cc/yylego/kratos-apm.svg?variant=adaptive)](https://starchart.cc/yylego/kratos-apm)
