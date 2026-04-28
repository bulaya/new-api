package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureDailyLoginRewardPointConversion(t *testing.T) {
	t.Helper()
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
}

func setUserPhoneForDailyLoginRewardTest(t *testing.T, userId int, phone string) {
	t.Helper()
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Update("phone", phone).Error)
}

func insertDailyLoginRewardUserForTest(t *testing.T, userId int, username string, phone string, quota int) {
	t.Helper()
	user := &User{
		Id:       userId,
		Username: username,
		Status:   common.UserStatusEnabled,
		Phone:    phone,
		Quota:    quota,
		AffCode:  username + "_aff",
	}
	require.NoError(t, DB.Create(user).Error)
}

func TestClaimClientDailyLoginRewardAwardsOncePerDay(t *testing.T) {
	truncateTables(t)
	configureDailyLoginRewardPointConversion(t)

	insertUserForPaymentGuardTest(t, 501, 0)
	setUserPhoneForDailyLoginRewardTest(t, 501, "13900000501")
	expectedQuota := PointsToQuota(ClientDailyLoginRewardPoints)

	status, err := ClaimClientDailyLoginReward(501)
	require.NoError(t, err)
	assert.True(t, status.Rewarded)
	assert.Equal(t, ClientDailyLoginRewardPoints, status.PointsAwarded)
	assert.Equal(t, expectedQuota, status.QuotaAwarded)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 501))

	status, err = ClaimClientDailyLoginReward(501)
	require.NoError(t, err)
	assert.False(t, status.Rewarded)
	assert.Equal(t, expectedQuota, status.QuotaAwarded)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 501))
}

func TestGetClientDailyLoginRewardStatus(t *testing.T) {
	truncateTables(t)
	configureDailyLoginRewardPointConversion(t)

	insertUserForPaymentGuardTest(t, 502, 0)
	setUserPhoneForDailyLoginRewardTest(t, 502, "13900000502")
	status, err := GetClientDailyLoginRewardStatus(502)
	require.NoError(t, err)
	assert.False(t, status.Rewarded)

	_, err = ClaimClientDailyLoginReward(502)
	require.NoError(t, err)

	status, err = GetClientDailyLoginRewardStatus(502)
	require.NoError(t, err)
	assert.True(t, status.Rewarded)
	assert.Equal(t, ClientDailyLoginRewardPoints, status.PointsAwarded)
	assert.Equal(t, PointsToQuota(ClientDailyLoginRewardPoints), status.QuotaAwarded)
}

func TestClaimClientDailyLoginRewardAwardsOncePerPhonePerDay(t *testing.T) {
	truncateTables(t)
	configureDailyLoginRewardPointConversion(t)

	const phone = "13900000503"
	insertDailyLoginRewardUserForTest(t, 503, "daily_reward_user_503", phone, 0)
	insertDailyLoginRewardUserForTest(t, 504, "daily_reward_user_504", phone, 0)

	expectedQuota := PointsToQuota(ClientDailyLoginRewardPoints)

	status, err := ClaimClientDailyLoginReward(503)
	require.NoError(t, err)
	assert.True(t, status.Rewarded)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 503))

	status, err = ClaimClientDailyLoginReward(504)
	require.NoError(t, err)
	assert.False(t, status.Rewarded)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 504))

	status, err = GetClientDailyLoginRewardStatus(504)
	require.NoError(t, err)
	assert.True(t, status.Rewarded)
}

func TestClaimClientDailyLoginRewardConcurrentAwardsOnce(t *testing.T) {
	truncateTables(t)
	configureDailyLoginRewardPointConversion(t)

	insertDailyLoginRewardUserForTest(t, 505, "daily_reward_user_505", "13900000505", 0)
	expectedQuota := PointsToQuota(ClientDailyLoginRewardPoints)

	const workers = 20
	var wg sync.WaitGroup
	results := make(chan ClientDailyLoginRewardStatus, workers)
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, err := ClaimClientDailyLoginReward(505)
			if err != nil {
				errs <- err
				return
			}
			results <- status
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	rewardedCount := 0
	for status := range results {
		if status.Rewarded {
			rewardedCount++
		}
	}
	assert.Equal(t, 1, rewardedCount)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 505))
}
