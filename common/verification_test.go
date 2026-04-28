package common

import "testing"

func TestConsumeCodeWithKeyIsSingleUse(t *testing.T) {
	key := "13900000000"
	code := "123456"
	RegisterVerificationCodeWithKey(key, code, SmsVerificationPurpose)

	if !ConsumeCodeWithKey(key, code, SmsVerificationPurpose) {
		t.Fatal("expected first consume to succeed")
	}
	if ConsumeCodeWithKey(key, code, SmsVerificationPurpose) {
		t.Fatal("expected second consume to fail")
	}
}
