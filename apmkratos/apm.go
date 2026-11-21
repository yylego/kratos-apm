package apmkratos

import (
	"github.com/yylego/elasticapm"
	"github.com/yylego/zaplog"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"
)

// ApmAgentVersion returns the version of the Elastic APM agent this package is built with
//
// ApmAgentVersion 返回本包编译时用的 Elastic APM agent 版本
func ApmAgentVersion() string {
	return apm.AgentVersion
}

// CheckApmAgentVersion reports if the given version equals the APM agent version of this package and of elasticapm
// A mismatch logs a warning and gives false
// Pass the apm.AgentVersion of the agent package that the application imports, at startup:
// it catches an application that imports the old go.elastic.co/apm (v1) while this package uses go.elastic.co/apm/v2
//
// CheckApmAgentVersion 检查传入的版本跟本包、以及 elasticapm 用的 APM agent 版本是否一致
// 不一致时打一条告警并返回 false
// 在启动时把应用自己引入的 agent 包的 apm.AgentVersion 传进来：
// 应用引入的是旧的 go.elastic.co/apm（v1）、而本包用的是 go.elastic.co/apm/v2 时，就能查出来
func CheckApmAgentVersion(version string) bool {
	if agentVersion := apm.AgentVersion; version != agentVersion {
		zaplog.LOGGER.LOG.Warn("apm agent version mismatch", zap.String("arg_version", version), zap.String("pkg_version", agentVersion))
		return false
	}
	// elasticapm checks its own agent version too
	// elasticapm 再检查它自己的 agent 版本
	return elasticapm.CheckApmAgentVersion(version)
}
