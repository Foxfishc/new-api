package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

const (
	GroupRuleWindowDays     = 7
	GroupRuleThreshold      = 10.0
	GroupRuleDefaultGroup   = "default"
	GroupRuleQualifiedGroup = "svip"
)

type GroupRuleStatus struct {
	Role                    int     `json:"-"`
	UserId                  int     `json:"user_id"`
	CurrentGroup            string  `json:"current_group"`
	Qualified               bool    `json:"qualified"`
	Changed                 bool    `json:"changed"`
	ConsumptionAverage      float64 `json:"consumption_average"`
	RechargeAverage         float64 `json:"recharge_average"`
	ConsumptionAverageQuota float64 `json:"consumption_average_quota"`
	RechargeAverageQuota    float64 `json:"recharge_average_quota"`
	ThresholdQuota          float64 `json:"threshold_quota"`
	WindowDays              int     `json:"window_days"`
	Currency                string  `json:"currency"`
	CurrencySymbol          string  `json:"currency_symbol"`
	QualifiedGroup          string  `json:"qualified_group"`
	FallbackGroup           string  `json:"fallback_group"`
	EvaluatedAt             int64   `json:"evaluated_at"`
}

func groupRuleQuotaToCurrency(quota int64) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	amount := float64(quota) / common.QuotaPerUnit
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		return amount * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeCustom:
		return amount * operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		return float64(quota)
	default:
		return amount
	}
}

func addGroupRuleQuota(total *int64, value int64) {
	if value <= 0 {
		return
	}
	if value > math.MaxInt64-*total {
		*total = math.MaxInt64
		return
	}
	*total += value
}

func creditedQuotaForTopUp(topUp TopUp) (int64, error) {
	if topUp.PaymentProvider == PaymentProviderCreem {
		return topUp.Amount, nil
	}
	amount := decimal.NewFromInt(topUp.Amount)
	if topUp.PaymentProvider == PaymentProviderStripe {
		amount = decimal.NewFromFloat(topUp.Money)
	}
	quota, err := common.WalletQuotaFromDecimalStrict(amount.Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	return int64(quota), err
}

func groupRuleCurrencyInfo() (string, string) {
	displayType := operation_setting.GetQuotaDisplayType()
	switch displayType {
	case operation_setting.QuotaDisplayTypeCNY:
		return displayType, "¥"
	case operation_setting.QuotaDisplayTypeCustom:
		return displayType, operation_setting.GetCurrencySymbol()
	case operation_setting.QuotaDisplayTypeTokens:
		return displayType, ""
	default:
		return operation_setting.QuotaDisplayTypeUSD, "$"
	}
}

func groupRuleCutoff(now int64) int64 {
	return now - int64(GroupRuleWindowDays*24*60*60)
}

// GetGroupRuleStatus calculates the rolling seven-day averages. Averages are
// divided by seven even when some days have no activity, matching the rule's
// “seven-day average” definition.
func GetGroupRuleStatus(userId int) (GroupRuleStatus, error) {
	if userId <= 0 {
		return GroupRuleStatus{}, errors.New("invalid user id")
	}
	var user User
	if err := DB.Where("id = ?", userId).First(&user).Error; err != nil {
		return GroupRuleStatus{}, err
	}
	cutoff := groupRuleCutoff(common.GetTimestamp())
	var consumedQuota int64
	if err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ?", userId, LogTypeConsume, cutoff).
		Select("COALESCE(SUM(quota), 0)").Scan(&consumedQuota).Error; err != nil {
		return GroupRuleStatus{}, err
	}
	var topUps []TopUp
	if err := DB.Where("user_id = ? AND status = ? AND complete_time >= ?", userId, common.TopUpStatusSuccess, cutoff).Find(&topUps).Error; err != nil {
		return GroupRuleStatus{}, err
	}
	var creditedQuota int64
	for _, topUp := range topUps {
		quota, err := creditedQuotaForTopUp(topUp)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to convert top-up %d for group rule: %v", topUp.Id, err))
			continue
		}
		addGroupRuleQuota(&creditedQuota, quota)
	}
	var redemptions []Redemption
	if err := DB.Where("used_user_id = ? AND status = ? AND redeemed_time >= ?", userId, common.RedemptionCodeStatusUsed, cutoff).Find(&redemptions).Error; err != nil {
		return GroupRuleStatus{}, err
	}
	for _, redemption := range redemptions {
		addGroupRuleQuota(&creditedQuota, int64(redemption.Quota))
	}
	currency, symbol := groupRuleCurrencyInfo()
	consumptionAverage := groupRuleQuotaToCurrency(consumedQuota) / GroupRuleWindowDays
	rechargeAverage := groupRuleQuotaToCurrency(creditedQuota) / GroupRuleWindowDays
	return GroupRuleStatus{
		Role:                    user.Role,
		UserId:                  userId,
		CurrentGroup:            user.Group,
		Qualified:               consumptionAverage >= GroupRuleThreshold || rechargeAverage >= GroupRuleThreshold,
		ConsumptionAverage:      consumptionAverage,
		RechargeAverage:         rechargeAverage,
		ConsumptionAverageQuota: float64(consumedQuota) / GroupRuleWindowDays,
		RechargeAverageQuota:    float64(creditedQuota) / GroupRuleWindowDays,
		ThresholdQuota:          thresholdQuotaForDisplay(),
		WindowDays:              GroupRuleWindowDays,
		Currency:                currency,
		CurrencySymbol:          symbol,
		QualifiedGroup:          GroupRuleQualifiedGroup,
		FallbackGroup:           GroupRuleDefaultGroup,
		EvaluatedAt:             common.GetTimestamp(),
	}, nil
}

func thresholdQuotaForDisplay() float64 {
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		return GroupRuleThreshold
	}
	rate := operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
	if rate <= 0 {
		rate = 1
	}
	return GroupRuleThreshold / rate * common.QuotaPerUnit
}

// EvaluateAndApplyGroupRule updates only users currently in the two managed
// groups. Other groups remain untouched, preserving manual/admin assignments.
func EvaluateAndApplyGroupRule(userId int) (GroupRuleStatus, error) {
	status, err := GetGroupRuleStatus(userId)
	if err != nil {
		return status, err
	}
	target := GroupRuleDefaultGroup
	if status.Qualified {
		target = GroupRuleQualifiedGroup
	}
	if status.Role >= common.RoleAdminUser {
		return status, nil
	}
	if !isManagedGroup(status.CurrentGroup) || status.CurrentGroup == target {
		return status, nil
	}
	result := DB.Model(&User{}).
		Where("id = ?", userId).
		Where(clause.Eq{Column: clause.Column{Name: "group"}, Value: status.CurrentGroup}).
		Update("group", target)
	if result.Error != nil {
		return status, result.Error
	}
	if result.RowsAffected == 1 {
		status.Changed = true
		status.CurrentGroup = target
		if err := RefreshUserGroupCache(userId); err != nil {
			return status, fmt.Errorf("refresh user group cache: %w", err)
		}
	}
	return status, nil
}

// EvaluateManagedGroupRules evaluates all users whose group is managed by the
// rule. It is used by the daily system task so a user can be downgraded after
// the rolling window expires even without opening the profile page.
func EvaluateManagedGroupRules() (int, error) {
	var users []User
	if err := DB.Select("id").Where(clause.IN{
		Column: clause.Column{Name: "group"},
		Values: []interface{}{GroupRuleDefaultGroup, GroupRuleQualifiedGroup},
	}).Find(&users).Error; err != nil {
		return 0, err
	}
	evaluated := 0
	for _, user := range users {
		if _, err := EvaluateAndApplyGroupRule(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to evaluate automatic group rule for user %d: %v", user.Id, err))
			continue
		}
		evaluated++
	}
	return evaluated, nil
}

func isManagedGroup(group string) bool {
	group = strings.TrimSpace(group)
	return group == GroupRuleDefaultGroup || group == GroupRuleQualifiedGroup
}
