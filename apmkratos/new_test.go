package apmkratos

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yylego/elasticapm"
	"go.elastic.co/apm/v2"
)

// The agent sends in the background, so starting and closing it needs no APM service at this address
// agent 在后台发送数据，启动和关闭它都不需要这个地址上有 APM 服务
const apmServerAddress = "http://127.0.0.1:8200"

func TestInitialize(t *testing.T) {
	require.NoError(t, Initialize(&elasticapm.Config{
		Environment:    "testing",
		ServerUrl:      apmServerAddress,
		ServiceName:    "article-service",
		ServiceVersion: "1.0.0",
	}))
	defer Close()

	require.True(t, apm.DefaultTracer().Active())
	require.Equal(t, "testing", os.Getenv("ELASTIC_APM_ENVIRONMENT"))
	require.Equal(t, apmServerAddress, os.Getenv("ELASTIC_APM_SERVER_URL"))
}

// TestInitialize_MissingServerAddress checks a config with no APM service address is refused
// TestInitialize_MissingServerAddress 检查没填 APM 服务地址的配置会被拒绝
func TestInitialize_MissingServerAddress(t *testing.T) {
	require.Error(t, Initialize(&elasticapm.Config{
		Environment:    "testing",
		ServiceName:    "article-service",
		ServiceVersion: "1.0.0",
	}))
}

// TestInitializeWithOptions checks two things: with Override off the variables set ahead stay unchanged,
// and each func in setEnvs runs once, at a time when the config is in the environment
//
// TestInitializeWithOptions 检查两件事：Override 关着时，事先设好的环境变量保持原样；
// setEnvs 里的函数各执行一次，执行时配置已经写进环境变量
func TestInitializeWithOptions(t *testing.T) {
	t.Setenv("ELASTIC_APM_ENVIRONMENT", "staging")
	t.Setenv("ELASTIC_APM_SERVER_URL", "")

	var addressesSeen []string
	require.NoError(t, InitializeWithOptions(&elasticapm.Config{
		Environment:    "testing",
		ServerUrl:      apmServerAddress,
		ServiceName:    "article-service",
		ServiceVersion: "1.0.0",
	}, &elasticapm.EnvOption{Override: false}, func() {
		addressesSeen = append(addressesSeen, os.Getenv("ELASTIC_APM_SERVER_URL"))
	}))
	defer Close()

	require.True(t, apm.DefaultTracer().Active())
	require.Equal(t, []string{apmServerAddress}, addressesSeen)
	require.Equal(t, "staging", os.Getenv("ELASTIC_APM_ENVIRONMENT"))
}

// TestClose checks Close sends out the transaction that ended ahead of it
// TestClose 检查 Close 之前结束的事务，由 Close 发出去
func TestClose(t *testing.T) {
	recording := startRecording(t)

	apm.DefaultTracer().StartTransaction("GET /articles", "request").End()
	Close()

	require.Len(t, recording.Payloads().Transactions, 1)
}
