package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordCyberPolicyEventAutoBansAtConfiguredThreshold(t *testing.T) {
	user := &User{
		Username: "cyber-auto-ban",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		DB.Unscoped().Delete(&User{}, user.Id)
		DB.Where("user_id = ?", user.Id).Delete(&CyberPolicyEvent{})
	})

	first, err := RecordCyberPolicyEvent(user.Id, "cyber-request-1", 1, "model-a", 2, true, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(1), first.Count)
	assert.False(t, first.AutoBanned)

	second, err := RecordCyberPolicyEvent(user.Id, "cyber-request-2", 1, "model-a", 2, true, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(2), second.Count)
	assert.True(t, second.AutoBanned)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, updated.Status)
}

func TestRecordCyberPolicyEventIsIdempotentForRequestId(t *testing.T) {
	user := &User{
		Username: "cyber-idempotent",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		DB.Unscoped().Delete(&User{}, user.Id)
		DB.Where("user_id = ?", user.Id).Delete(&CyberPolicyEvent{})
	})

	first, err := RecordCyberPolicyEvent(user.Id, "same-request", 1, "model-a", 2, true, 2)
	require.NoError(t, err)
	second, err := RecordCyberPolicyEvent(user.Id, "same-request", 1, "model-a", 2, true, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(1), first.Count)
	assert.Equal(t, int64(1), second.Count)

	var events int64
	require.NoError(t, DB.Model(&CyberPolicyEvent{}).Where("user_id = ?", user.Id).Count(&events).Error)
	assert.Equal(t, int64(1), events)
}

func TestRecordCyberPolicyEventNeverBansAdministrators(t *testing.T) {
	user := &User{
		Username: "cyber-admin",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleAdminUser,
	}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		DB.Unscoped().Delete(&User{}, user.Id)
		DB.Where("user_id = ?", user.Id).Delete(&CyberPolicyEvent{})
	})

	result, err := RecordCyberPolicyEvent(user.Id, "admin-request", 1, "model-a", 2, true, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Count)
	assert.False(t, result.AutoBanned)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	assert.Equal(t, common.UserStatusEnabled, updated.Status)
}
