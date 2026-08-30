package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// CyberPolicyEvent is the durable per-request audit counter used by the
// optional automatic user-ban rule. It intentionally lives in the primary
// database rather than the configurable log database so the counter remains
// available when logs are sent to ClickHouse or are cleaned up.
type CyberPolicyEvent struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"index;uniqueIndex:idx_cyber_policy_user_request,priority:1"`
	RequestId    string `json:"request_id" gorm:"index;uniqueIndex:idx_cyber_policy_user_request,priority:2"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	ChannelId    int    `json:"channel_id" gorm:"default:0"`
	ModelName    string `json:"model_name" gorm:"default:''"`
	TokenId      int    `json:"token_id" gorm:"default:0"`
	CountAtEvent int64  `json:"count_at_event" gorm:"default:0"`
	AutoBanned   bool   `json:"auto_banned"`
}

type CyberPolicyEventResult struct {
	Count      int64
	Threshold  int
	AutoBanned bool
}

// RecordCyberPolicyEvent stores one event and atomically evaluates the
// cumulative threshold. A non-empty request ID is treated as an idempotency
// key so a transport retry or duplicate terminal event cannot increment the
// user's count twice.
func RecordCyberPolicyEvent(userId int, requestId string, channelId int, modelName string, tokenId int, enabled bool, threshold int) (CyberPolicyEventResult, error) {
	result := CyberPolicyEventResult{Threshold: threshold}
	if userId <= 0 {
		return result, nil
	}
	requestId = strings.TrimSpace(requestId)
	if requestId == "" {
		requestId = common.NewRequestId()
	}
	if threshold < 1 {
		threshold = 0
		result.Threshold = 0
	}

	autoBanApplied := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx.Unscoped()).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		var event CyberPolicyEvent
		existing := false
		err := tx.Where("user_id = ? AND request_id = ?", userId, requestId).First(&event).Error
		if err == nil {
			existing = true
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if !existing {
			event = CyberPolicyEvent{
				UserId:    userId,
				RequestId: requestId,
				CreatedAt: common.GetTimestamp(),
				ChannelId: channelId,
				ModelName: modelName,
				TokenId:   tokenId,
			}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&CyberPolicyEvent{}).Where("user_id = ?", userId).Count(&result.Count).Error; err != nil {
			return err
		}
		result.AutoBanned = event.AutoBanned

		if enabled && threshold > 0 && result.Count >= int64(threshold) {
			if user.Role < common.RoleAdminUser && user.Status == common.UserStatusEnabled {
				if _, err := IncrementUserAuthVersionWithTx(tx, userId); err != nil {
					return err
				}
				updated := tx.Unscoped().Model(&User{}).
					Where("id = ? AND status = ?", userId, common.UserStatusEnabled).
					Update("status", common.UserStatusDisabled)
				if updated.Error != nil {
					return updated.Error
				}
				autoBanApplied = updated.RowsAffected == 1
				result.AutoBanned = autoBanApplied
			}
		}

		event.CountAtEvent = result.Count
		event.AutoBanned = result.AutoBanned
		updates := map[string]interface{}{"auto_banned": result.AutoBanned}
		if !existing {
			updates["count_at_event"] = result.Count
		}
		return tx.Model(&event).Updates(updates).Error
	})
	if err != nil {
		return result, fmt.Errorf("record cyber policy event: %w", err)
	}
	if autoBanApplied {
		if err := PublishUserAuthCache(userId); err != nil {
			return result, fmt.Errorf("publish auto-banned user cache: %w", err)
		}
		if _, err := RevokeAllUserSessions(userId, "cyber_policy_auto_ban"); err != nil {
			return result, fmt.Errorf("revoke auto-banned user sessions: %w", err)
		}
		if err := InvalidateUserTokensCache(userId); err != nil {
			return result, fmt.Errorf("invalidate auto-banned user tokens: %w", err)
		}
	}
	return result, nil
}
