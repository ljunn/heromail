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
	digest := sha256.Sum256([]byte(canonical(values, false)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("生成测试签名失败：%v", err)
	}
	values.Set("sign", base64.StdEncoding.EncodeToString(signature))
	if !verifyAlipay(values, publicPEM) {
		t.Fatal("支付宝 RSA2 正确签名未通过")
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
	parsed, err := parsePrivateKey(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})))
	if err != nil || parsed.N.Cmp(privateKey.N) != 0 {
		t.Fatalf("解析 PKCS8 私钥失败：%v", err)
	}
}
