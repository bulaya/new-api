package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestPointsQuotaRoundTrip(t *testing.T) {
	originalRate := operation_setting.USDExchangeRate
	originalPointsPerCNY := operation_setting.GetPaymentSetting().PointsPerCNY
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		operation_setting.USDExchangeRate = originalRate
		operation_setting.GetPaymentSetting().PointsPerCNY = originalPointsPerCNY
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	operation_setting.USDExchangeRate = 7.3
	operation_setting.GetPaymentSetting().PointsPerCNY = 200
	common.QuotaPerUnit = 500 * 1000

	quota := PointsToQuota(200)
	if quota <= 0 {
		t.Fatalf("expected positive quota, got %d", quota)
	}

	points := QuotaToPoints(int64(quota))
	if points != 200 {
		t.Fatalf("expected round trip points to equal 200, got %d", points)
	}
}

func TestResolveSubscriptionDisplayPoints(t *testing.T) {
	plan := &SubscriptionPlan{
		DisplayPoints: 4000,
		TotalAmount:   100000,
	}
	sub := &UserSubscription{
		AmountTotal: 100000,
		AmountUsed:  25000,
	}

	total, used, remaining, unlimited := ResolveSubscriptionDisplayPoints(plan, sub)
	if unlimited {
		t.Fatalf("expected limited subscription")
	}
	if total != 4000 {
		t.Fatalf("expected total points 4000, got %d", total)
	}
	if used != 1000 {
		t.Fatalf("expected used points 1000, got %d", used)
	}
	if remaining != 3000 {
		t.Fatalf("expected remaining points 3000, got %d", remaining)
	}
}
