package webhook

import (
	"net"
	"testing"
)

func TestPrivateAddressDetection(t *testing.T) {
	tests := []struct {
		address string
		private bool
	}{
		{address: "127.0.0.1", private: true},
		{address: "10.1.2.3", private: true},
		{address: "172.16.0.1", private: true},
		{address: "192.168.1.1", private: true},
		{address: "169.254.169.254", private: true},
		{address: "::1", private: true},
		{address: "8.8.8.8", private: false},
		{address: "2606:4700:4700::1111", private: false},
	}
	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			if got := isPrivate(net.ParseIP(test.address)); got != test.private {
				t.Fatalf("isPrivate(%s) = %v，期望 %v", test.address, got, test.private)
			}
		})
	}
}
