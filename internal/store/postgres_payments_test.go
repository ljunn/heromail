package store

import "testing"

func TestPaymentOrderCanCompleteAfterLocalExpiry(t *testing.T) {
	for _, status := range []string{"pending", "paid", "expired"} {
		if !paymentOrderCanComplete(status) {
			t.Fatalf("支付状态 %q 应允许在验签回调后入账", status)
		}
	}
	for _, status := range []string{"completed", "canceled"} {
		if paymentOrderCanComplete(status) {
			t.Fatalf("支付状态 %q 不应重复或越权入账", status)
		}
	}
}
