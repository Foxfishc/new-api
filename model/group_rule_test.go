package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateAndApplyGroupRuleUsesRechargeCredits(t *testing.T) {
	user := &User{Username: "group-rule-recharge", Password: "password", Status: common.UserStatusEnabled, Group: GroupRuleDefaultGroup}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		DB.Unscoped().Delete(&User{}, user.Id)
		DB.Where("used_user_id = ?", user.Id).Delete(&Redemption{})
	})

	originalType := operation_setting.GetGeneralSetting().QuotaDisplayType
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() { operation_setting.GetGeneralSetting().QuotaDisplayType = originalType })

	// Seventy USD credited via a redemption code over the seven-day window
	// means a seven-day average of ten, which qualifies the user.
	require.NoError(t, DB.Create(&Redemption{
		UserId: 999, UsedUserId: user.Id, Quota: int(common.QuotaPerUnit * 70),
		Status: common.RedemptionCodeStatusUsed, RedeemedTime: common.GetTimestamp(),
	}).Error)

	status, err := EvaluateAndApplyGroupRule(user.Id)
	require.NoError(t, err)
	assert.True(t, status.Qualified)
	assert.True(t, status.Changed)
	assert.Equal(t, GroupRuleQualifiedGroup, status.CurrentGroup)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	assert.Equal(t, GroupRuleQualifiedGroup, updated.Group)
}

func TestEvaluateAndApplyGroupRuleUsesConsumptionOrRecharge(t *testing.T) {
	user := &User{Username: "group-rule-consume", Password: "password", Status: common.UserStatusEnabled, Group: GroupRuleDefaultGroup}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		DB.Unscoped().Delete(&User{}, user.Id)
		DB.Where("user_id = ?", user.Id).Delete(&Log{})
		DB.Where("user_id = ?", user.Id).Delete(&Redemption{})
	})

	require.NoError(t, DB.Create(&Log{
		UserId: user.Id, Type: LogTypeConsume, Quota: int(common.QuotaPerUnit * GroupRuleThreshold * GroupRuleWindowDays), CreatedAt: common.GetTimestamp(),
	}).Error)
	status, err := EvaluateAndApplyGroupRule(user.Id)
	require.NoError(t, err)
	assert.True(t, status.Qualified)
	assert.Equal(t, 10.0, status.ConsumptionAverage)
}

func TestEvaluateAndApplyGroupRuleLeavesManualGroupsUntouched(t *testing.T) {
	user := &User{Username: "group-rule-manual", Password: "password", Status: common.UserStatusEnabled, Group: "partner"}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(&User{}, user.Id) })

	status, err := EvaluateAndApplyGroupRule(user.Id)
	require.NoError(t, err)
	assert.False(t, status.Changed)
	assert.Equal(t, "partner", status.CurrentGroup)
}
