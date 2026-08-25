package payment

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"testing"
)

func TestEasyPaySignAndVerify(t *testing.T) {
	values := url.Values{
		"pid":          {"1001"},
		"type":         {"alipay"},
		"out_trade_no": {"PAY001"},
		"money":        {"50.00"},
		"name":         {"HeroMail 余额充值"},
		"sign_type":    {"MD5"},
	}
	values.Set("sign", signEasyPay(values, "merchant-secret"))
	if !verifyEasyPay(values, "merchant-secret") {
		t.Fatal("易支付正确签名未通过")
	}
	values.Set("money", "500.00")
	if verifyEasyPay(values, "merchant-secret") {
		t.Fatal("易支付被篡改的金额通过了验签")
	}
}

func TestAlipayRSA2Verify(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试密钥失败：%v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("序列化测试公钥失败：%v", err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	values := url.Values{
		"app_id":       {"2026000000000001"},
		"out_trade_no": {"PAY002"},
		"trade_no":     {"202608170001"},
		"trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"88.00"},
		"sign_type":    {"RSA2"},
	}
	digest := sha256.Sum256([]byte(canonical(values, true)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("生成测试签名失败：%v", err)
	}
	values.Set("sign", base64.StdEncoding.EncodeToString(signature))
	if !verifyAlipay(values, publicPEM) {
		t.Fatal("支付宝 RSA2 正确签名未通过")
	}
	values.Set("sign_type", "RSA1")
	if !verifyAlipay(values, publicPEM) {
		t.Fatal("支付宝 sign_type 不应参与回调签名校验")
	}
	values.Set("total_amount", "89.00")
	if verifyAlipay(values, publicPEM) {
		t.Fatal("支付宝被篡改的金额通过了验签")
	}
}

func TestParseAlipayPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试密钥失败：%v", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("序列化测试私钥失败：%v", err)
	}
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "PKCS8 PEM",
			value: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})),
		},
		{
			name:  "PKCS8 Base64",
			value: base64.StdEncoding.EncodeToString(pkcs8),
		},
		{
			name:  "PKCS1 PEM",
			value: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parsePrivateKey(tt.value)
			if err != nil || parsed.N.Cmp(privateKey.N) != 0 {
				t.Fatalf("解析 %s 私钥失败：%v", tt.name, err)
			}
		})
	}
}

func TestValidateAlipayProviderConfigAcceptsBarePublicKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试私钥失败：%v", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("序列化测试私钥失败：%v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("序列化测试公钥失败：%v", err)
	}
	config := map[string]string{
		"app_id":      "2026000000000000",
		"private_key": base64.StdEncoding.EncodeToString(privateDER),
		"public_key":  base64.StdEncoding.EncodeToString(publicDER),
	}
	if err := ValidateProviderConfig("alipay", config); err != nil {
		t.Fatalf("裸 Base64 支付宝密钥不应被拒绝：%v", err)
	}
	config["private_key"] += "应用公钥"
	if err := ValidateProviderConfig("alipay", config); err == nil {
		t.Fatal("混入非 Base64 文本的私钥应被拒绝")
	}
}
