package apmkratos

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2"
)

func TestApmAgentVersion(t *testing.T) {
	require.Equal(t, apm.AgentVersion, ApmAgentVersion())
}

func TestCheckApmAgentVersion(t *testing.T) {
	require.True(t, CheckApmAgentVersion(apm.AgentVersion))
	// 1.15.0 is a version of the old go.elastic.co/apm (v1) agent
	// 1.15.0 是旧的 go.elastic.co/apm（v1）agent 的版本
	require.False(t, CheckApmAgentVersion("1.15.0"))
}
