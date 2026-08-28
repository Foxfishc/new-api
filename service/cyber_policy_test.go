package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectCyberPolicy(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		hit     bool
		message string
	}{
		{name: "top-level error", payload: `{"error":{"code":"cyber_policy","message":"blocked"}}`, hit: true, message: "blocked"},
		{name: "response-wrapped error", payload: `{"response":{"error":{"code":"CYBER_POLICY","message":"  blocked  "}}}`, hit: true, message: "blocked"},
		{name: "nested error", payload: `{"error":{"error":{"code":"cyber_policy"}}}`, hit: true},
		{name: "ordinary policy", payload: `{"error":{"code":"content_policy"}}`, hit: false},
		{name: "plain text", payload: "cyber_policy", hit: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hit, message := DetectCyberPolicy([]byte(test.payload))
			assert.Equal(t, test.hit, hit)
			assert.Equal(t, test.message, message)
		})
	}
}

func TestCyberPolicyErrorIsNonRetryable(t *testing.T) {
	oldAutomaticDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = oldAutomaticDisable })
	oldStatusRules := operation_setting.AutomaticDisableKeywordsToString()
	t.Cleanup(func() { operation_setting.AutomaticDisableKeywordsFromString(oldStatusRules) })
	operation_setting.AutomaticDisableKeywordsFromString("cyber_policy")

	err := CyberPolicyError(&CyberPolicyMark{Message: "blocked", UpstreamStatus: http.StatusBadRequest})
	require.NotNil(t, err)
	assert.True(t, IsCyberPolicyError(err))
	assert.True(t, types.IsSkipRetryError(err))
	assert.False(t, ShouldDisableChannel(err))
}

func TestRelayErrorHandlerCyberPolicyReturnsStructuredCode(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"cyber_policy","message":"blocked"}}`)),
	}
	err := RelayErrorHandler(context.Background(), resp, false)
	require.NotNil(t, err)
	assert.Equal(t, types.ErrorCodeCyberPolicy, err.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(err))
}

func TestMarkCyberPolicyFirstWins(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	MarkCyberPolicy(c, CyberPolicyMark{Message: "first", Body: "one", UpstreamStatus: http.StatusOK})
	MarkCyberPolicy(c, CyberPolicyMark{Message: "second", Body: "two", UpstreamStatus: http.StatusBadRequest})
	mark := GetCyberPolicy(c)
	require.NotNil(t, mark)
	assert.Equal(t, "first", mark.Message)
	assert.Equal(t, "one", mark.Body)
	assert.Equal(t, http.StatusOK, mark.UpstreamStatus)
}
