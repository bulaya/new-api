package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

func quotaToPointsDecimal(quota int64) decimal.Decimal {
	if quota <= 0 {
		return decimal.Zero
	}
	rate := operation_setting.USDExchangeRate
	if rate <= 0 {
		rate = 1
	}
	return decimal.NewFromInt(quota).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(rate)).
		Mul(decimal.NewFromInt(operation_setting.GetPointsPerCNY()))
}

func pointsToQuotaDecimal(points int64) decimal.Decimal {
	if points <= 0 {
		return decimal.Zero
	}
	rate := operation_setting.USDExchangeRate
	if rate <= 0 {
		rate = 1
	}
	return decimal.NewFromInt(points).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Div(decimal.NewFromFloat(rate)).
		Div(decimal.NewFromInt(operation_setting.GetPointsPerCNY()))
}

func QuotaToPoints(quota int64) int64 {
	return quotaToPointsDecimal(quota).Round(0).IntPart()
}

func PointsToQuota(points int64) int {
	return int(pointsToQuotaDecimal(points).Round(0).IntPart())
}

func ResolvePlanDisplayPoints(plan *SubscriptionPlan) int64 {
	if plan == nil {
		return 0
	}
	if plan.DisplayPoints > 0 {
		return plan.DisplayPoints
	}
	if plan.TotalAmount <= 0 {
		return 0
	}
	return QuotaToPoints(plan.TotalAmount)
}

func ResolveSubscriptionDisplayPoints(plan *SubscriptionPlan, sub *UserSubscription) (int64, int64, int64, bool) {
	if plan == nil || sub == nil {
		return 0, 0, 0, false
	}
	displayTotal := ResolvePlanDisplayPoints(plan)
	if sub.AmountTotal <= 0 {
		return displayTotal, 0, displayTotal, true
	}
	if displayTotal <= 0 {
		displayTotal = QuotaToPoints(sub.AmountTotal)
	}
	if displayTotal <= 0 {
		return 0, 0, 0, false
	}
	used := decimal.NewFromInt(displayTotal).
		Mul(decimal.NewFromInt(sub.AmountUsed)).
		Div(decimal.NewFromInt(sub.AmountTotal)).
		Round(0).
		IntPart()
	if used < 0 {
		used = 0
	}
	if used > displayTotal {
		used = displayTotal
	}
	remaining := displayTotal - used
	if remaining < 0 {
		remaining = 0
	}
	return displayTotal, used, remaining, false
}
