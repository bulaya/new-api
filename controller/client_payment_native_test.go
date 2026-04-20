package controller

import "testing"

func TestExtractRedirectURL(t *testing.T) {
	html := `<script>window.location.replace('/pay/wxpay/2026042011522967909/')</script>`

	got := extractRedirectURL(html)
	want := "/pay/wxpay/2026042011522967909/"
	if got != want {
		t.Fatalf("extractRedirectURL() = %q, want %q", got, want)
	}
}

func TestExtractHTMLValueFromEpayCashierPage(t *testing.T) {
	html := `
		<html>
			<body>
				<script>var code_url = 'weixin://wxpay/bizpayurl?pr=CVcbnJnz3';</script>
				<button data-clipboard-text="weixin://wxpay/bizpayurl?pr=CVcbnJnz3"></button>
				<dd id="billId">2026042011522967909</dd>
				<dd id="productName">月套餐</dd>
			</body>
		</html>
	`

	if got := extractHTMLValue(html, epayCodeURLPattern); got != "weixin://wxpay/bizpayurl?pr=CVcbnJnz3" {
		t.Fatalf("extractHTMLValue(code_url) = %q", got)
	}
	if got := extractHTMLValue(html, epayClipboardURLPattern); got != "weixin://wxpay/bizpayurl?pr=CVcbnJnz3" {
		t.Fatalf("extractHTMLValue(clipboard) = %q", got)
	}
	if got := extractHTMLValue(html, epayProviderTradePattern); got != "2026042011522967909" {
		t.Fatalf("extractHTMLValue(billId) = %q", got)
	}
	if got := extractHTMLValue(html, epayProductNamePattern); got != "月套餐" {
		t.Fatalf("extractHTMLValue(productName) = %q", got)
	}
}
