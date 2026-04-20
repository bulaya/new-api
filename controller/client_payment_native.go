package controller

import (
	"context"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type ClientNativePaymentResponse struct {
	Provider         string  `json:"provider"`
	OrderType        string  `json:"order_type"`
	TradeNo          string  `json:"trade_no"`
	PaymentMethod    string  `json:"payment_method"`
	Status           string  `json:"status"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	Subject          string  `json:"subject"`
	CashierURL       string  `json:"cashier_url,omitempty"`
	QRCodeData       string  `json:"qr_code_data,omitempty"`
	QRCodeImage      string  `json:"qr_code_image,omitempty"`
	ProviderTradeNo  string  `json:"provider_trade_no,omitempty"`
	CreatedTime      int64   `json:"created_time,omitempty"`
	PointAmount      int64   `json:"point_amount,omitempty"`
	PlanID           int     `json:"plan_id,omitempty"`
}

type ClientPaymentStatusResponse struct {
	Provider      string  `json:"provider"`
	OrderType     string  `json:"order_type"`
	TradeNo       string  `json:"trade_no"`
	PaymentMethod string  `json:"payment_method"`
	Status        string  `json:"status"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Subject       string  `json:"subject"`
	CompleteTime  int64   `json:"complete_time,omitempty"`
	PointAmount   int64   `json:"point_amount,omitempty"`
	PlanID        int     `json:"plan_id,omitempty"`
}

var (
	epayRedirectURLPattern   = regexp.MustCompile(`window\.location\.replace\(['"]([^'"]+)['"]\)`)
	epayCodeURLPattern       = regexp.MustCompile(`var\s+code_url\s*=\s*'([^']+)'`)
	epayClipboardURLPattern  = regexp.MustCompile(`data-clipboard-text="([^"]+)"`)
	epayProviderTradePattern = regexp.MustCompile(`<dd id="billId">([^<]+)</dd>`)
	epayProductNamePattern   = regexp.MustCompile(`<dd id="productName">([^<]+)</dd>`)
)

func GetClientPaymentStatus(c *gin.Context) {
	userId := c.GetInt("id")
	tradeNo := strings.TrimSpace(c.Query("trade_no"))
	if userId <= 0 || tradeNo == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if order := model.GetSubscriptionOrderByTradeNo(tradeNo); order != nil {
		if order.UserId != userId {
			common.ApiErrorMsg(c, "订单不存在")
			return
		}
		subject := "订阅支付"
		if plan, err := model.GetSubscriptionPlanById(order.PlanId); err == nil && plan != nil {
			subject = plan.Title
		}
		common.ApiSuccess(c, ClientPaymentStatusResponse{
			Provider:      "epay",
			OrderType:     "subscription",
			TradeNo:       order.TradeNo,
			PaymentMethod: order.PaymentMethod,
			Status:        order.Status,
			Amount:        order.Money,
			Currency:      "CNY",
			Subject:       subject,
			CompleteTime:  order.CompleteTime,
			PlanID:        order.PlanId,
		})
		return
	}

	if topup := model.GetTopUpByTradeNo(tradeNo); topup != nil {
		if topup.UserId != userId {
			common.ApiErrorMsg(c, "订单不存在")
			return
		}
		subject := "积分充值"
		if topup.ProductType == "points" && topup.PointsAmount > 0 {
			subject = "积分充值"
		}
		common.ApiSuccess(c, ClientPaymentStatusResponse{
			Provider:      "epay",
			OrderType:     "points",
			TradeNo:       topup.TradeNo,
			PaymentMethod: topup.PaymentMethod,
			Status:        topup.Status,
			Amount:        topup.Money,
			Currency:      "CNY",
			Subject:       subject,
			CompleteTime:  topup.CompleteTime,
			PointAmount:   topup.PointsAmount,
		})
		return
	}

	common.ApiErrorMsg(c, "订单不存在")
}

func buildNativeEpayPaymentResponse(
	ctx context.Context,
	orderType string,
	paymentMethod string,
	tradeNo string,
	amount float64,
	subject string,
	cashierBaseURL string,
	cashierParams map[string]string,
) (*ClientNativePaymentResponse, error) {
	submitURL, err := buildSignedPaymentURL(cashierBaseURL, cashierParams)
	if err != nil {
		return nil, err
	}

	cashierURL, paymentPageHTML, err := resolveEpayCashierPage(ctx, submitURL)
	if err != nil {
		return nil, err
	}

	codeURL := firstNonEmpty(
		extractHTMLValue(paymentPageHTML, epayCodeURLPattern),
		extractHTMLValue(paymentPageHTML, epayClipboardURLPattern),
	)
	productName := firstNonEmpty(extractHTMLValue(paymentPageHTML, epayProductNamePattern), subject)
	providerTradeNo := extractHTMLValue(paymentPageHTML, epayProviderTradePattern)

	result := &ClientNativePaymentResponse{
		Provider:        "epay",
		OrderType:       orderType,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentMethod,
		Status:          common.TopUpStatusPending,
		Amount:          amount,
		Currency:        "CNY",
		Subject:         productName,
		CashierURL:      cashierURL,
		ProviderTradeNo: providerTradeNo,
	}

	if strings.HasPrefix(codeURL, "data:image/") || strings.HasPrefix(codeURL, "http://") || strings.HasPrefix(codeURL, "https://") {
		result.QRCodeImage = codeURL
	} else {
		result.QRCodeData = codeURL
	}

	return result, nil
}

func buildSignedPaymentURL(baseURL string, params map[string]string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func resolveEpayCashierPage(ctx context.Context, submitURL string) (string, string, error) {
	redirectHTML, err := fetchRemoteHTML(ctx, submitURL)
	if err != nil {
		return "", "", err
	}

	redirectTarget := extractRedirectURL(redirectHTML)
	if redirectTarget == "" {
		return submitURL, redirectHTML, nil
	}

	submitParsed, err := url.Parse(submitURL)
	if err != nil {
		return "", "", err
	}
	redirectParsed, err := url.Parse(redirectTarget)
	if err != nil {
		return "", "", err
	}
	cashierURL := submitParsed.ResolveReference(redirectParsed).String()
	paymentHTML, err := fetchRemoteHTML(ctx, cashierURL)
	if err != nil {
		return "", "", err
	}
	return cashierURL, paymentHTML, nil
}

func fetchRemoteHTML(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "MyClaw/1.0 Native Payment Parser")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func extractRedirectURL(pageHTML string) string {
	matches := epayRedirectURLPattern.FindStringSubmatch(pageHTML)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(stdhtml.UnescapeString(matches[1]))
}

func extractHTMLValue(pageHTML string, pattern *regexp.Regexp) string {
	matches := pattern.FindStringSubmatch(pageHTML)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(stdhtml.UnescapeString(matches[1]))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
