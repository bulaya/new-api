package model

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ClientDailyLoginRewardPoints int64 = 200

type ClientDailyLoginReward struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int    `json:"user_id" gorm:"not null;uniqueIndex:idx_client_daily_login_reward_user_date"`
	Phone         string `json:"phone" gorm:"type:varchar(20);not null;uniqueIndex:idx_client_daily_login_reward_phone_date"`
	RewardDate    string `json:"reward_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_client_daily_login_reward_user_date;uniqueIndex:idx_client_daily_login_reward_phone_date"`
	PointsAwarded int64  `json:"points_awarded" gorm:"not null"`
	QuotaAwarded  int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
}

type ClientDailyLoginRewardStatus struct {
	Rewarded      bool   `json:"rewarded"`
	RewardDate    string `json:"reward_date"`
	PointsAwarded int64  `json:"points_awarded"`
	QuotaAwarded  int    `json:"quota_awarded"`
}

func (ClientDailyLoginReward) TableName() string {
	return "client_daily_login_rewards"
}

func currentClientDailyLoginRewardDate() string {
	return time.Now().Format("2006-01-02")
}

func NewClientDailyLoginRewardStatus(rewarded bool, rewardDate string, quotaAwarded int) ClientDailyLoginRewardStatus {
	return ClientDailyLoginRewardStatus{
		Rewarded:      rewarded,
		RewardDate:    rewardDate,
		PointsAwarded: ClientDailyLoginRewardPoints,
		QuotaAwarded:  quotaAwarded,
	}
}

func GetClientDailyLoginRewardStatus(userId int) (ClientDailyLoginRewardStatus, error) {
	rewardDate := currentClientDailyLoginRewardDate()
	quotaAwarded := PointsToQuota(ClientDailyLoginRewardPoints)
	user, err := GetUserById(userId, false)
	if err != nil {
		return NewClientDailyLoginRewardStatus(false, rewardDate, quotaAwarded), err
	}
	if user.Phone == "" {
		return NewClientDailyLoginRewardStatus(false, rewardDate, quotaAwarded), nil
	}
	var count int64
	err = DB.Model(&ClientDailyLoginReward{}).
		Where("reward_date = ? AND (user_id = ? OR phone = ?)", rewardDate, userId, user.Phone).
		Count(&count).Error
	return NewClientDailyLoginRewardStatus(count > 0, rewardDate, quotaAwarded), err
}

func ClaimClientDailyLoginReward(userId int) (ClientDailyLoginRewardStatus, error) {
	rewardDate := currentClientDailyLoginRewardDate()
	quotaAwarded := PointsToQuota(ClientDailyLoginRewardPoints)
	status := NewClientDailyLoginRewardStatus(false, rewardDate, quotaAwarded)
	if userId <= 0 || quotaAwarded <= 0 {
		return status, nil
	}
	user, err := GetUserById(userId, false)
	if err != nil {
		return status, err
	}
	if user.Phone == "" {
		return status, nil
	}

	reward := &ClientDailyLoginReward{
		UserId:        userId,
		Phone:         user.Phone,
		RewardDate:    rewardDate,
		PointsAwarded: ClientDailyLoginRewardPoints,
		QuotaAwarded:  quotaAwarded,
		CreatedAt:     time.Now().Unix(),
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(reward)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", quotaAwarded)).Error; err != nil {
			return err
		}
		status.Rewarded = true
		return nil
	})
	if err != nil {
		return status, err
	}

	if status.Rewarded {
		go func() {
			_ = cacheIncrUserQuota(userId, int64(quotaAwarded))
		}()
	}

	return status, nil
}
