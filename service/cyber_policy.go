package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	CyberPolicyCode    = "cyber_policy"
	cyberPolicyMarkKey = "cyber_policy_mark"
)

// CyberPolicyMark stores the structured upstream evidence for one request.
// The request context remains the source of truth for user identity; the
// upstream payload contributes only the policy code/message and status.
type CyberPolicyMark struct {
	Code           string
	Message        string
	Body           string
	UpstreamStatus int
	EventCount     int64
	AutoBanned     bool
	eventRecorded  bool
}

// DetectCyberPolicy recognizes the provider-side structured policy signal used
// by Codex/OpenAI-compatible endpoints. It deliberately does not classify
// arbitrary natural-language output, avoiding broad false positives.
func DetectCyberPolicy(payload []byte) (bool, string) {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return false, ""
	}
	paths := []string{"error.code", "response.error.code", "error.error.code"}
	matched := false
	for _, path := range paths {
		if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(payload, path).String()), CyberPolicyCode) {
			matched = true
			break
		}
	}
	if !matched {
		return false, ""
	}
	messagePaths := []string{"error.message", "response.error.message", "error.error.message"}
	for _, path := range messagePaths {
		if message := strings.TrimSpace(gjson.GetBytes(payload, path).String()); message != "" {
			return true, message
		}
	}
	return true, ""
}

// MarkCyberPolicy records the first cyber_policy signal in a request. A
// first-wins mark prevents retries/stream terminal events from double writing
// the same audit event.
func MarkCyberPolicy(c *gin.Context, mark CyberPolicyMark) {
	if c == nil || GetCyberPolicy(c) != nil {
		return
	}
	mark.Code = CyberPolicyCode
	mark.Message = strings.TrimSpace(mark.Message)
	mark.Body = truncateCyberPolicyBody(mark.Body)
	if mark.UpstreamStatus == 0 {
		mark.UpstreamStatus = http.StatusBadGateway
	}
	c.Set(cyberPolicyMarkKey, &mark)
}

func GetCyberPolicy(c *gin.Context) *CyberPolicyMark {
	if c == nil {
		return nil
	}
	value, ok := c.Get(cyberPolicyMarkKey)
	if !ok {
		return nil
	}
	mark, ok := value.(*CyberPolicyMark)
	if !ok || mark == nil {
		return nil
	}
	return mark
}

func MarkCyberPolicyFromError(c *gin.Context, err *types.NewAPIError) {
	if err == nil || !IsCyberPolicyError(err) {
		return
	}
	MarkCyberPolicy(c, CyberPolicyMark{
		Code:           CyberPolicyCode,
		Message:        err.Error(),
		UpstreamStatus: err.StatusCode,
	})
}

// HandleCyberPolicyEvent persists the request-level event and applies the
// optional cumulative auto-ban rule. It is intentionally idempotent per Gin
// context because streaming and terminal error paths can both observe the
// same upstream policy event.
func HandleCyberPolicyEvent(c *gin.Context, mark *CyberPolicyMark) {
	if c == nil || mark == nil || mark.eventRecorded {
		return
	}
	mark.eventRecorded = true
	result, err := model.RecordCyberPolicyEvent(
		c.GetInt("id"),
		c.GetString(common.RequestIdKey),
		c.GetInt("channel_id"),
		c.GetString("original_model"),
		c.GetInt("token_id"),
		setting.CyberAutoBanEnabled,
		setting.CyberAutoBanThreshold,
	)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to process cyber policy event for user %d: %v", c.GetInt("id"), err))
		return
	}
	mark.EventCount = result.Count
	mark.AutoBanned = result.AutoBanned
}

func IsCyberPolicyError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return strings.EqualFold(string(err.GetErrorCode()), CyberPolicyCode)
}

// CyberPolicyError builds a non-retryable error for stream paths that have
// already forwarded the provider's policy event to the client.
func CyberPolicyError(mark *CyberPolicyMark) *types.NewAPIError {
	message := "upstream cyber-security policy blocked the request"
	status := http.StatusBadRequest
	if mark != nil {
		if mark.Message != "" {
			message = mark.Message
		}
		if mark.UpstreamStatus >= 100 && mark.UpstreamStatus <= 599 {
			status = mark.UpstreamStatus
		}
	}
	return types.NewOpenAIError(errors.New(message), types.ErrorCodeCyberPolicy, status, types.ErrOptionWithSkipRetry())
}

// RecordCyberPolicyErrorLog writes a single existing LogTypeError row for a
// stream/WebSocket path that completed without returning a NewAPIError. The
// identity fields are read from the authenticated request context, never from
// the upstream response body.
func RecordCyberPolicyErrorLog(c *gin.Context, mark *CyberPolicyMark) {
	if c == nil || mark == nil {
		return
	}
	HandleCyberPolicyEvent(c, mark)
	userID := c.GetInt("id")
	channelID := c.GetInt("channel_id")
	modelName := c.GetString("original_model")
	tokenName := c.GetString("token_name")
	tokenID := c.GetInt("token_id")
	group := c.GetString("group")
	useTimeSeconds := 0
	start := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if !start.IsZero() {
		useTimeSeconds = int(time.Since(start).Seconds())
	}
	other := map[string]interface{}{
		"cyber_policy":         true,
		"error_type":           CyberPolicyCode,
		"error_code":           CyberPolicyCode,
		"status_code":          mark.UpstreamStatus,
		"upstream_status_code": mark.UpstreamStatus,
		"cyber_policy_count":   mark.EventCount,
		"cyber_auto_banned":    mark.AutoBanned,
		"admin_info": map[string]interface{}{
			"cyber_policy_message": common.LocalLogPreview(common.MaskSensitiveInfo(mark.Message)),
			"upstream_cyber_body":  common.LocalLogPreview(common.MaskSensitiveInfo(mark.Body)),
		},
	}
	model.RecordErrorLog(c, userID, channelID, modelName, tokenName,
		fmt.Sprintf("status_code=%d, cyber_policy: %s", mark.UpstreamStatus, common.MaskSensitiveInfo(mark.Message)),
		tokenID, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), group, other)
}

func truncateCyberPolicyBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	const maxRunes = 4096
	runes := []rune(body)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "... [truncated]"
	}
	return body
}
