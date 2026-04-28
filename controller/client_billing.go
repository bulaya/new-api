package controller

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type ClientPointPackage struct {
	AmountCNY             int64   `json:"amount_cny"`
	PaymentAmount         float64 `json:"payment_amount"`
	OriginalPaymentAmount float64 `json:"original_payment_amount"`
	Points                int64   `json:"points"`
	DiscountRate          float64 `json:"discount_rate"`
}

type ClientBillingCatalogResponse struct {
	Currency          string                `json:"currency"`
	CurrencySymbol    string                `json:"currency_symbol"`
	PointUnitName     string                `json:"point_unit_name"`
	PointsPerCNY      int64                 `json:"points_per_cny"`
	PaymentMethods    []map[string]string   `json:"payment_methods"`
	SubscriptionPlans []SubscriptionPlanDTO `json:"subscription_plans"`
	PointPackages     []ClientPointPackage  `json:"point_packages"`
}

type ClientSubscriptionStatus struct {
	SubscriptionID  int    `json:"subscription_id"`
	PlanID          int    `json:"plan_id"`
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	BadgeText       string `json:"badge_text"`
	Status          string `json:"status"`
	EndTime         int64  `json:"end_time"`
	TotalPoints     int64  `json:"total_points"`
	UsedPoints      int64  `json:"used_points"`
	RemainingPoints int64  `json:"remaining_points"`
	Unlimited       bool   `json:"unlimited"`
}

type ClientBillingSelfResponse struct {
	User                     ClientUserInfo                     `json:"user"`
	PointUnitName            string                             `json:"point_unit_name"`
	WalletPoints             int64                              `json:"wallet_points"`
	SubscriptionPoints       int64                              `json:"subscription_points"`
	TotalPoints              int64                              `json:"total_points"`
	HasUnlimitedSubscription bool                               `json:"has_unlimited_subscription"`
	ActiveSubscriptions      []ClientSubscriptionStatus         `json:"active_subscriptions"`
	DailyLoginReward         model.ClientDailyLoginRewardStatus `json:"daily_login_reward"`
}

type ClientPointTopupRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

func buildClientPointPackages() []ClientPointPackage {
	options := append([]int(nil), operation_setting.GetPaymentSetting().AmountOptions...)
	sort.Ints(options)
	pointsPerCNY := operation_setting.GetPointsPerCNY()
	packages := make([]ClientPointPackage, 0, len(options))
	for _, amount := range options {
		if amount <= 0 {
			continue
		}
		discountRate := 1.0
		if discount, ok := operation_setting.GetPaymentSetting().AmountDiscount[amount]; ok && discount > 0 {
			discountRate = discount
		}
		paymentAmount := decimal.NewFromInt(int64(amount)).
			Mul(decimal.NewFromFloat(discountRate)).
			Round(2).
			InexactFloat64()
		packages = append(packages, ClientPointPackage{
			AmountCNY:             int64(amount),
			PaymentAmount:         paymentAmount,
			OriginalPaymentAmount: float64(amount),
			Points:                int64(amount) * pointsPerCNY,
			DiscountRate:          discountRate,
		})
	}
	return packages
}

func GetClientBillingCatalog(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{Plan: p})
	}
	common.ApiSuccess(c, ClientBillingCatalogResponse{
		Currency:          "CNY",
		CurrencySymbol:    "¥",
		PointUnitName:     operation_setting.GetPointUnitName(),
		PointsPerCNY:      operation_setting.GetPointsPerCNY(),
		PaymentMethods:    operation_setting.PayMethods,
		SubscriptionPlans: result,
		PointPackages:     buildClientPointPackages(),
	})
}

func GetClientBillingSelf(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	activeSubs, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		activeSubs = []model.SubscriptionSummary{}
	}

	subscriptionStatuses := make([]ClientSubscriptionStatus, 0, len(activeSubs))
	subscriptionPoints := int64(0)
	hasUnlimited := false
	for _, item := range activeSubs {
		sub := item.Subscription
		if sub == nil {
			continue
		}
		plan, err := model.GetSubscriptionPlanById(sub.PlanId)
		if err != nil {
			continue
		}
		totalPoints, usedPoints, remainingPoints, unlimited := model.ResolveSubscriptionDisplayPoints(plan, sub)
		if unlimited {
			hasUnlimited = true
		}
		subscriptionPoints += remainingPoints
		subscriptionStatuses = append(subscriptionStatuses, ClientSubscriptionStatus{
			SubscriptionID:  sub.Id,
			PlanID:          sub.PlanId,
			Title:           plan.Title,
			Subtitle:        plan.Subtitle,
			BadgeText:       plan.BadgeText,
			Status:          sub.Status,
			EndTime:         sub.EndTime,
			TotalPoints:     totalPoints,
			UsedPoints:      usedPoints,
			RemainingPoints: remainingPoints,
			Unlimited:       unlimited,
		})
	}

	walletPoints := model.QuotaToPoints(int64(user.Quota))
	dailyReward, err := model.GetClientDailyLoginRewardStatus(userId)
	if err != nil {
		common.SysLog("GetClientBillingSelf: failed to get daily login reward status: " + err.Error())
		dailyReward = model.NewClientDailyLoginRewardStatus(false, "", model.PointsToQuota(model.ClientDailyLoginRewardPoints))
	}
	common.ApiSuccess(c, ClientBillingSelfResponse{
		User: ClientUserInfo{
			Id:           user.Id,
			Phone:        user.Phone,
			Quota:        user.Quota,
			UsedQuota:    user.UsedQuota,
			RequestCount: user.RequestCount,
		},
		PointUnitName:            operation_setting.GetPointUnitName(),
		WalletPoints:             walletPoints,
		SubscriptionPoints:       subscriptionPoints,
		TotalPoints:              walletPoints + subscriptionPoints,
		HasUnlimitedSubscription: hasUnlimited,
		ActiveSubscriptions:      subscriptionStatuses,
		DailyLoginReward:         dailyReward,
	})
}

func ClientPointTopupRequestEpay(c *gin.Context) {
	var req ClientPointTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < int64(operation_setting.MinTopUp) {
		common.ApiErrorMsg(c, fmt.Sprintf("充值金额不能小于 %d 元", operation_setting.MinTopUp))
		return
	}
	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}

	discountRate := 1.0
	if discount, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(req.Amount)]; ok && discount > 0 {
		discountRate = discount
	}
	payMoney := decimal.NewFromInt(req.Amount).
		Mul(decimal.NewFromFloat(discountRate)).
		Round(2).
		InexactFloat64()
	if payMoney < 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}

	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}

	userId := c.GetInt("id")
	pointsAmount := req.Amount * operation_setting.GetPointsPerCNY()
	callBackAddress := service.GetCallbackAddress()
	returnUrl, _ := url.Parse(system_setting.ServerAddress + "/console/topup")
	notifyUrl, _ := url.Parse(callBackAddress + "/api/user/epay/notify")
	tradeNo := fmt.Sprintf("CLPTUSR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("POINTS:%d", pointsAmount),
		Money:          strconv.FormatFloat(payMoney, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}

	topUp := &model.TopUp{
		UserId:        userId,
		Amount:        req.Amount,
		PointsAmount:  pointsAmount,
		ProductType:   "points",
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: req.PaymentMethod,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	payment, err := buildNativeEpayPaymentResponse(
		c.Request.Context(),
		"points",
		req.PaymentMethod,
		tradeNo,
		payMoney,
		fmt.Sprintf("%s充值", operation_setting.GetPointUnitName()),
		uri,
		params,
	)
	if err != nil {
		common.ApiErrorMsg(c, "解析支付二维码失败")
		return
	}
	payment.CreatedTime = topUp.CreateTime
	payment.PointAmount = pointsAmount

	common.ApiSuccess(c, payment)
}
