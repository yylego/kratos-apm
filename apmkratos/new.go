package apmkratos

import (
	"github.com/yylego/elasticapm"
	"github.com/yylego/elasticapm/apmzaplog"
	"github.com/yylego/erero"
	"go.elastic.co/apm/v2"
)

// Initialize starts the APM agent from apmConfig with the default environment option,
// and sends the logs of the agent itself to zap
//
// Initialize 按 apmConfig 和默认的环境变量选项启动 APM agent，并把 agent 自身的日志接到 zap
func Initialize(apmConfig *elasticapm.Config) error {
	if err := InitializeWithOptions(apmConfig, elasticapm.NewEnvOption()); err != nil {
		return erero.Wro(err)
	}
	apm.DefaultTracer().SetLogger(apmzaplog.NewLog())
	return nil
}

// InitializeWithOptions starts the APM agent from apmConfig with the given environment option
// Each func in setEnvs runs once the config is written into the environment, ahead of creating the agent,
// so it is the place to set more environment variables
//
// InitializeWithOptions 按 apmConfig 和传入的环境变量选项启动 APM agent
// setEnvs 里的函数在配置写进环境变量之后、创建 agent 之前各执行一次，想再设置别的环境变量就写在里面
func InitializeWithOptions(apmConfig *elasticapm.Config, envOption *elasticapm.EnvOption, setEnvs ...func()) error {
	if err := elasticapm.InitializeWithOptions(apmConfig, envOption, setEnvs...); err != nil {
		return erero.Wro(err)
	}
	return nil
}

// Close sends the APM data left in the queue and stops the agent
// Invoke it when the application exits, with defer in the main func
//
// Close 把还没发出去的 APM 数据发完，并停掉 agent
// 在程序退出时调用，在 main 函数里用 defer
func Close() {
	elasticapm.Close()
}
