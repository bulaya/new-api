package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type PaymentSetting struct {
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠
	PointsPerCNY   int64           `json:"points_per_cny"`
	PointUnitName  string          `json:"point_unit_name"`
}

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:  []int{10, 20, 50, 100, 200, 500},
	AmountDiscount: map[int]float64{},
	PointsPerCNY:   200,
	PointUnitName:  "积分",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func GetPointsPerCNY() int64 {
	if paymentSetting.PointsPerCNY <= 0 {
		return 200
	}
	return paymentSetting.PointsPerCNY
}

func GetPointUnitName() string {
	name := strings.TrimSpace(paymentSetting.PointUnitName)
	if name == "" {
		return "积分"
	}
	return name
}
