package payment

import (
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

type Service struct {
	repository store.PaymentRepository
	publicURL  string
}

func New(repository store.PaymentRepository, publicURL string) *Service {
	return &Service{repository: repository, publicURL: strings.TrimRight(publicURL, "/")}
}

func (s *Service) Methods() ([]string, error) {
	providers, err := s.repository.ListEnabledPaymentProviders("alipay")
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return []string{}, nil
	}
	return []string{"alipay"}, nil
}

func (s *Service) Create(userID string, amount float64, method string, mobile bool) (domain.PaymentOrder, error) {
	providers, err := s.repository.ListEnabledPaymentProviders(method)
	if err != nil || len(providers) == 0 {
		return domain.PaymentOrder{}, store.ErrPaymentProviderNotFound
	}
	provider := providers[0]
	if err := ValidateProviderConfig(provider.Provider.Type, provider.Config); err != nil {
		return domain.PaymentOrder{}, err
	}
	order, err := s.repository.CreatePaymentOrder(userID, provider.Provider.ID, method, amount)
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	var payURL string
	switch provider.Provider.Type {
	case "easypay":
		payURL, err = s.createEasyPay(provider, order)
	case "alipay":
		payURL, err = s.createAlipay(provider, order, mobile)
	default:
		err = errors.New("不支持的支付服务商类型")
	}
	if err != nil {
		_, _ = s.repository.CancelPaymentOrder(userID, order.ID)
		return domain.PaymentOrder{}, err
	}
	if err := s.repository.SetPaymentOrderURL(order.ID, payURL); err != nil {
		return domain.PaymentOrder{}, err
	}
	order.PayURL = payURL
	return order, nil
}

// ValidateProviderConfig 在保存和创建支付单前校验服务商配置，避免把不可用的凭证留到用户点击支付时才失败。
func ValidateProviderConfig(providerType string, config map[string]string) error {
	switch providerType {
	case "easypay":
		if strings.TrimSpace(config["api_base"]) == "" || strings.TrimSpace(config["pid"]) == "" || strings.TrimSpace(config["pkey"]) == "" {
			return errors.New("易支付必须填写 API 地址、商户 ID 和商户密钥")
		}
		return nil
	case "alipay":
		if strings.TrimSpace(config["app_id"]) == "" || strings.TrimSpace(config["private_key"]) == "" || strings.TrimSpace(config["public_key"]) == "" {
			return errors.New("支付宝官方必须填写 AppID、应用私钥和支付宝公钥")
		}
		if _, err := parsePrivateKey(config["private_key"]); err != nil {
			return fmt.Errorf("支付宝应用私钥无效：%w", err)
		}
		if _, err := parsePublicKey(config["public_key"]); err != nil {
			return fmt.Errorf("支付宝公钥无效：%w", err)
		}
		return nil
	default:
		return errors.New("不支持的支付服务商类型")
	}
}

func (s *Service) Notify(providerType, providerID string, values url.Values) error {
	provider, err := s.repository.GetPaymentProviderSecret(providerID)
	if err != nil || provider.Provider.Type != providerType || !provider.Provider.Enabled {
		return store.ErrPaymentProviderNotFound
	}
	var orderID, tradeNo string
	var amount float64
	switch providerType {
	case "easypay":
		if !verifyEasyPay(values, provider.Config["pkey"]) || values.Get("trade_status") != "TRADE_SUCCESS" || values.Get("pid") != provider.Config["pid"] {
			return errors.New("易支付回调验签失败")
		}
		orderID, tradeNo = values.Get("out_trade_no"), values.Get("trade_no")
		amount, err = strconv.ParseFloat(values.Get("money"), 64)
	case "alipay":
		if values.Get("app_id") != provider.Config["app_id"] || !verifyAlipay(values, provider.Config["public_key"]) {
			return errors.New("支付宝回调验签失败")
		}
		status := values.Get("trade_status")
		if status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" {
			return errors.New("支付宝交易尚未成功")
		}
		orderID, tradeNo = values.Get("out_trade_no"), values.Get("trade_no")
		amount, err = strconv.ParseFloat(values.Get("total_amount"), 64)
	default:
		return errors.New("不支持的支付回调类型")
	}
	if err != nil || orderID == "" || tradeNo == "" {
		return errors.New("支付回调参数不完整")
	}
	_, err = s.repository.CompletePaymentOrder(orderID, tradeNo, amount)
	return err
}

func (s *Service) createEasyPay(provider store.PaymentProviderSecret, order domain.PaymentOrder) (string, error) {
	apiBase := provider.Config["api_base"]
	if apiBase == "" || provider.Config["pid"] == "" || provider.Config["pkey"] == "" || s.publicURL == "" {
		return "", errors.New("易支付配置不完整")
	}
	values := url.Values{
		"pid":          {provider.Config["pid"]},
		"type":         {order.Method},
		"out_trade_no": {order.ID},
		"notify_url":   {s.publicURL + "/api/v1/payment/webhook/easypay?provider_id=" + provider.Provider.ID},
		"return_url":   {s.publicURL + "/?payment_order=" + order.ID},
		"name":         {"HeroMail 余额充值"},
		"money":        {fmt.Sprintf("%.2f", order.Amount)},
	}
	if channelID := provider.Config["channel_id"]; channelID != "" {
		values.Set("cid", channelID)
	}
	values.Set("sign", signEasyPay(values, provider.Config["pkey"]))
	values.Set("sign_type", "MD5")
	separator := "?"
	if strings.Contains(apiBase, "?") {
		separator = "&"
	}
	return apiBase + separator + values.Encode(), nil
}

func (s *Service) createAlipay(provider store.PaymentProviderSecret, order domain.PaymentOrder, mobile bool) (string, error) {
	config := provider.Config
	if config["app_id"] == "" || config["private_key"] == "" || s.publicURL == "" {
		return "", errors.New("支付宝官方配置不完整")
	}
	gateway := config["gateway"]
	if gateway == "" {
		gateway = "https://openapi.alipay.com/gateway.do"
	}
	method, productCode := "alipay.trade.page.pay", "FAST_INSTANT_TRADE_PAY"
	if mobile {
		method, productCode = "alipay.trade.wap.pay", "QUICK_WAP_WAY"
	}
	bizContent, _ := json.Marshal(map[string]any{"out_trade_no": order.ID, "product_code": productCode, "total_amount": fmt.Sprintf("%.2f", order.Amount), "subject": "HeroMail 余额充值", "timeout_express": "30m"})
	values := url.Values{
		"app_id":      {config["app_id"]},
		"method":      {method},
		"format":      {"JSON"},
		"charset":     {"utf-8"},
		"sign_type":   {"RSA2"},
		"timestamp":   {time.Now().Format("2006-01-02 15:04:05")},
		"version":     {"1.0"},
		"notify_url":  {s.publicURL + "/api/v1/payment/webhook/alipay?provider_id=" + provider.Provider.ID},
		"return_url":  {s.publicURL + "/?payment_order=" + order.ID},
		"biz_content": {string(bizContent)},
	}
	privateKey, err := parsePrivateKey(config["private_key"])
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical(values, false)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	values.Set("sign", base64.StdEncoding.EncodeToString(signature))
	return gateway + "?" + values.Encode(), nil
}

func signEasyPay(values url.Values, key string) string {
	hash := md5.Sum([]byte(canonical(values, true) + key))
	return hex.EncodeToString(hash[:])
}

func verifyEasyPay(values url.Values, key string) bool {
	return strings.EqualFold(values.Get("sign"), signEasyPay(values, key))
}

func verifyAlipay(values url.Values, publicKeyText string) bool {
	publicKey, err := parsePublicKey(publicKeyText)
	if err != nil {
		return false
	}
	signature, err := base64.StdEncoding.DecodeString(values.Get("sign"))
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(canonical(values, false)))
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature) == nil
}

func canonical(values url.Values, excludeSignType bool) string {
	keys := make([]string, 0, len(values))
	for key, entries := range values {
		if key == "sign" || (excludeSignType && key == "sign_type") || len(entries) == 0 || entries[0] == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "&")
}

func parsePrivateKey(value string) (*rsa.PrivateKey, error) {
	keyBytes := []byte(strings.TrimSpace(value))
	if block, _ := pem.Decode(keyBytes); block != nil {
		keyBytes = block.Bytes
	} else {
		// 支付宝控制台常见的私钥是去掉 PEM 头尾的 Base64 字符串。
		decoded, err := decodeBase64Key(value)
		if err != nil {
			return nil, errors.New("支付宝应用私钥不是有效 PEM 或 Base64")
		}
		keyBytes = decoded
	}
	if key, err := x509.ParsePKCS8PrivateKey(keyBytes); err == nil {
		if privateKey, ok := key.(*rsa.PrivateKey); ok {
			return privateKey, nil
		}
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(keyBytes)
	if err != nil {
		return nil, errors.New("支付宝应用私钥不是有效 RSA 私钥")
	}
	return privateKey, nil
}

func parsePublicKey(value string) (*rsa.PublicKey, error) {
	keyBytes := []byte(strings.TrimSpace(value))
	if block, _ := pem.Decode(keyBytes); block != nil {
		keyBytes = block.Bytes
	} else {
		// 支付宝控制台也常见只粘贴 Base64 公钥正文的格式。
		decoded, err := decodeBase64Key(value)
		if err != nil {
			return nil, errors.New("支付宝公钥不是有效 PEM 或 Base64")
		}
		keyBytes = decoded
	}
	if key, err := x509.ParsePKIXPublicKey(keyBytes); err == nil {
		if publicKey, ok := key.(*rsa.PublicKey); ok {
			return publicKey, nil
		}
	}
	if publicKey, err := x509.ParsePKCS1PublicKey(keyBytes); err == nil {
		return publicKey, nil
	}
	certificate, err := x509.ParseCertificate(keyBytes)
	if err != nil {
		return nil, errors.New("支付宝公钥不是有效 RSA 公钥")
	}
	publicKey, ok := certificate.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("支付宝公钥类型不是 RSA")
	}
	return publicKey, nil
}

func decodeBase64Key(value string) ([]byte, error) {
	encoded := strings.Join(strings.Fields(value), "")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(encoded)
}
